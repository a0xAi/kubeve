package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ColumnOptions struct {
	Timestamp bool
	Namespace bool
	Status    bool
	Action    bool
	Resource  bool
	Aggregate bool
}

func NewTable(status string) *tview.Table {
	table := tview.NewTable().SetBorders(false).SetFixed(1, 0)
	table.SetSelectable(true, false)
	table.SetBorder(true).SetTitle(status)
	// table.SetBackgroundColor(0x00ff00)
	return table
}

func renderTableHeader(table *tview.Table, opts ColumnOptions) {
	col := 0
	if opts.Timestamp {
		label := "TIME"
		if opts.Aggregate {
			label = "LAST SEEN"
		}
		table.SetCell(0, col, tview.NewTableCell(label).
			SetSelectable(false).SetAttributes(tcell.AttrBold).SetExpansion(1))
		col++
	}
	if opts.Namespace {
		table.SetCell(0, col, tview.NewTableCell("NAMESPACE").
			SetSelectable(false).SetAttributes(tcell.AttrBold).SetExpansion(1))
		col++
	}
	if opts.Status {
		label := "STATUS"
		if opts.Aggregate {
			label = "COUNT"
		}
		table.SetCell(0, col, tview.NewTableCell(label).
			SetSelectable(false).SetAttributes(tcell.AttrBold).SetExpansion(1))
		col++
	}
	if opts.Action {
		label := "ACTION"
		table.SetCell(0, col, tview.NewTableCell(label).
			SetSelectable(false).SetAttributes(tcell.AttrBold).SetExpansion(1))
		col++
	}
	if opts.Resource {
		table.SetCell(0, col, tview.NewTableCell("RESOURCE").
			SetSelectable(false).SetAttributes(tcell.AttrBold).SetExpansion(2))
		col++
	}
	messageLabel := "MESSAGE"
	if opts.Aggregate {
		messageLabel = "LAST MESSAGE"
	}
	table.SetCell(0, col, tview.NewTableCell(messageLabel).
		SetSelectable(false).SetAttributes(tcell.AttrBold).SetExpansion(5))
}

func renderRow(table *tview.Table, row int, parts []string, opts ColumnOptions) {
	col := 0
	if opts.Timestamp {
		table.SetCell(row, col, tview.NewTableCell(strings.TrimSpace(parts[0])).SetExpansion(1))
		col++
	}
	if opts.Namespace {
		table.SetCell(row, col, tview.NewTableCell(strings.TrimSpace(parts[4])).SetExpansion(1))
		col++
	}
	if opts.Status {
		statusText := strings.TrimSpace(parts[2])
		statusColor := "[white]"
		switch statusText {
		case "Warning":
			statusColor = "[yellow]"
		}
		table.SetCell(row, col, tview.NewTableCell(fmt.Sprintf("%s%s", statusColor, statusText)).SetExpansion(1))
		col++
	}
	if opts.Action {
		actionText := strings.TrimSpace(parts[3])
		actionColor := "[white]"
		switch actionText {
		case "Created", "SuccessfulCreate", "Completed":
			actionColor = "[green]"
		case "Started", "Pulled", "Pulling":
			actionColor = "[blue]"
		case "Killing", "BackOff", "Unhealthy", "FailedToRetrieveImagePullSecret":
			actionColor = "[red]"
		}
		table.SetCell(row, col, tview.NewTableCell(fmt.Sprintf("%s%s", actionColor, actionText)).
			SetExpansion(1).SetTextColor(tcell.ColorWhite))
		col++
	}
	if opts.Resource {
		table.SetCell(row, col, tview.NewTableCell(strings.TrimSpace(parts[1])).SetExpansion(2))
		col++
	}
	table.SetCell(row, col, tview.NewTableCell(strings.TrimSpace(parts[5])).SetExpansion(5))
}

func matchesFilter(line string, filterText string) bool {
	return strings.Contains(line, filterText)
}

func filterEvents(events []string, filterText string) []string {
	filtered := make([]string, 0, len(events))
	for _, line := range events {
		if matchesFilter(line, filterText) {
			filtered = append(filtered, line)
		}
	}
	return filtered
}

type aggregatedEvent struct {
	namespace   string
	resource    string
	reason      string
	lastMessage string
	lastSeen    time.Time
	lastType    string
	count       int
}

func aggregateEvents(events []string) []string {
	groups := make(map[string]*aggregatedEvent, len(events))
	for _, line := range events {
		parts := strings.SplitN(line, "│", 6)
		if len(parts) != 6 {
			continue
		}

		lastSeenText := strings.TrimSpace(parts[0])
		resource := strings.TrimSpace(parts[1])
		eventType := strings.TrimSpace(parts[2])
		reason := strings.TrimSpace(parts[3])
		namespace := strings.TrimSpace(parts[4])
		message := strings.TrimSpace(parts[5])

		key := namespace + "|" + resource + "|" + reason
		group, exists := groups[key]
		if !exists {
			group = &aggregatedEvent{
				namespace: namespace,
				resource:  resource,
				reason:    reason,
				lastType:  eventType,
			}
			groups[key] = group
		}
		group.count++

		parsedTime, err := time.Parse(time.RFC3339, lastSeenText)
		if err != nil {
			parsedTime = time.Time{}
		}
		if group.lastSeen.IsZero() || parsedTime.After(group.lastSeen) {
			group.lastSeen = parsedTime
			group.lastType = eventType
			group.lastMessage = message
		}
	}

	summary := make([]*aggregatedEvent, 0, len(groups))
	for _, group := range groups {
		summary = append(summary, group)
	}
	sort.Slice(summary, func(i, j int) bool {
		if summary[i].count != summary[j].count {
			return summary[i].count > summary[j].count
		}
		if !summary[i].lastSeen.Equal(summary[j].lastSeen) {
			return summary[i].lastSeen.After(summary[j].lastSeen)
		}
		if summary[i].namespace != summary[j].namespace {
			return summary[i].namespace < summary[j].namespace
		}
		if summary[i].resource != summary[j].resource {
			return summary[i].resource < summary[j].resource
		}
		return summary[i].reason < summary[j].reason
	})

	lines := make([]string, 0, len(summary))
	for _, group := range summary {
		lastSeenText := ""
		if group.lastSeen.IsZero() {
			lastSeenText = "-"
		} else {
			lastSeenText = group.lastSeen.Format(time.RFC3339)
		}
		lines = append(lines, fmt.Sprintf("%-25s │ %-60s │ %-10s │ %-20s │ %-10s │ %s",
			lastSeenText,
			group.resource,
			strconv.Itoa(group.count),
			group.reason,
			group.namespace,
			group.lastMessage,
		))
	}

	return lines
}

func renderTableContent(table *tview.Table, events []string, filterText string, opts ColumnOptions) {
	row := 1
	for _, line := range filterEvents(events, filterText) {
		parts := strings.SplitN(line, "│", 6)
		if len(parts) == 6 {
			renderRow(table, row, parts, opts)
			row++
		}
	}
}

func renderTable(table *tview.Table, events []string, filterText string, opts ColumnOptions) {
	table.Clear()
	renderTableHeader(table, opts)
	renderTableContent(table, events, filterText, opts)
}
