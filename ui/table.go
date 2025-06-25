package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ColumnOptions struct {
	Timestamp bool
	Namespace bool
	Status    bool
	Action    bool
	Resource  bool
}

func NewTable(status string, bgColor tcell.Color) *tview.Table {
	table := tview.NewTable().SetBorders(false).SetFixed(1, 0)
	table.SetSelectable(true, false)
	table.SetBorder(true).SetTitle(status)
	table.SetBackgroundColor(bgColor)
	return table
}

func renderTableHeader(table *tview.Table, opts ColumnOptions) {
	col := 0
	if opts.Timestamp {
		table.SetCell(0, col, tview.NewTableCell("TIME").
			SetSelectable(false).SetAttributes(tcell.AttrBold).SetExpansion(1))
		col++
	}
	if opts.Namespace {
		table.SetCell(0, col, tview.NewTableCell("NAMESPACE").
			SetSelectable(false).SetAttributes(tcell.AttrBold).SetExpansion(1))
		col++
	}
	if opts.Status {
		table.SetCell(0, col, tview.NewTableCell("STATUS").
			SetSelectable(false).SetAttributes(tcell.AttrBold).SetExpansion(1))
		col++
	}
	if opts.Action {
		table.SetCell(0, col, tview.NewTableCell("ACTION").
			SetSelectable(false).SetAttributes(tcell.AttrBold).SetExpansion(1))
		col++
	}
	if opts.Resource {
		table.SetCell(0, col, tview.NewTableCell("RESOURCE").
			SetSelectable(false).SetAttributes(tcell.AttrBold).SetExpansion(2))
		col++
	}
	table.SetCell(0, col, tview.NewTableCell("MESSAGE").
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

func renderTableContent(table *tview.Table, events []string, filterText string, opts ColumnOptions) {
	row := 1
	for _, line := range events {
		if strings.Contains(line, filterText) {
			parts := strings.SplitN(line, "│", 6)
			if len(parts) == 6 {
				renderRow(table, row, parts, opts)
				row++
			}
		}
	}
}

func renderTable(table *tview.Table, events []string, filterText string, opts ColumnOptions) {
	table.Clear()
	renderTableHeader(table, opts)
	renderTableContent(table, events, filterText, opts)
}
