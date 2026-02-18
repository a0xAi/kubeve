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

func messageColumnWidth(tableWidth int, opts ColumnOptions) int {
	if tableWidth <= 0 {
		return 80
	}

	columns := 1 // message column
	expansionTotal := 5
	if opts.Timestamp {
		columns++
		expansionTotal++
	}
	if opts.Namespace {
		columns++
		expansionTotal++
	}
	if opts.Status {
		columns++
		expansionTotal++
	}
	if opts.Action {
		columns++
		expansionTotal++
	}
	if opts.Resource {
		columns++
		expansionTotal += 2
	}

	separatorWidth := (columns - 1) * 3 // " │ "
	usable := tableWidth - separatorWidth
	if usable < 20 {
		return 20
	}

	width := (usable * 5) / expansionTotal
	if width < 20 {
		return 20
	}
	return width
}

func wrapLine(text string, width int) []string {
	if width <= 0 || len(text) <= width {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	lines := make([]string, 0, len(words))
	current := ""
	flush := func() {
		if current != "" {
			lines = append(lines, current)
			current = ""
		}
	}

	for _, word := range words {
		if len(word) > width {
			flush()
			for len(word) > width {
				lines = append(lines, word[:width])
				word = word[width:]
			}
			current = word
			continue
		}

		if current == "" {
			current = word
			continue
		}
		candidate := current + " " + word
		if len(candidate) <= width {
			current = candidate
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	flush()

	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func wrapMessage(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	paragraphs := strings.Split(text, "\n")
	lines := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		trimmed := strings.TrimSpace(paragraph)
		if trimmed == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, wrapLine(trimmed, width)...)
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
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

func renderTableContent(
	table *tview.Table,
	events []string,
	filterText string,
	opts ColumnOptions,
	wrapMessages bool,
	tableWidth int,
) []int {
	rowToEvent := make([]int, 0, len(events))
	row := 1
	msgWidth := messageColumnWidth(tableWidth, opts)
	for eventIdx, line := range filterEvents(events, filterText) {
		parts := strings.SplitN(line, "│", 6)
		if len(parts) == 6 {
			if !wrapMessages {
				renderRow(table, row, parts, opts)
				rowToEvent = append(rowToEvent, eventIdx)
				row++
				continue
			}

			wrapped := wrapMessage(strings.TrimSpace(parts[5]), msgWidth)
			if len(wrapped) == 0 {
				wrapped = []string{""}
			}

			first := append([]string(nil), parts...)
			first[5] = wrapped[0]
			renderRow(table, row, first, opts)
			rowToEvent = append(rowToEvent, eventIdx)
			row++

			for _, cont := range wrapped[1:] {
				renderRow(table, row, []string{"", "", "", "", "", cont}, opts)
				rowToEvent = append(rowToEvent, eventIdx)
				row++
			}
		}
	}
	return rowToEvent
}

func renderTable(
	table *tview.Table,
	events []string,
	filterText string,
	opts ColumnOptions,
	wrapMessages bool,
	tableWidth int,
) []int {
	table.Clear()
	renderTableHeader(table, opts)
	return renderTableContent(table, events, filterText, opts, wrapMessages, tableWidth)
}
