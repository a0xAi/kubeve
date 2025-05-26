package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func DetailsModal(app *tview.Application, frame *tview.Frame, table *tview.Table, parts []string) {
	if len(parts) == 6 {
		timeStr := strings.TrimSpace(parts[0])
		resource := strings.TrimSpace(parts[1])
		status := strings.TrimSpace(parts[2])
		action := strings.TrimSpace(parts[3])
		namespace := strings.TrimSpace(parts[4])
		message := strings.TrimSpace(parts[5])

		defaultStatusColour := "[white]"
		switch status {
		case "Warning":
			defaultStatusColour = "[yellow]"
		}

		defaultActionColour := "[white]"
		switch action {
		case "Created", "SuccessfulCreate", "Completed":
			defaultActionColour = "[green]"
		case "Started", "Pulled", "Pulling":
			defaultActionColour = "[blue]"
		case "Killing", "BackOff", "Unhealthy", "FailedToRetrieveImagePullSecret":
			defaultActionColour = "[red]"
		}

		detail := fmt.Sprintf(
			"[blue]Time:      [white]%s\n"+
				"[blue]Resource:  [white]%s\n"+
				"[blue]Namespace: [white]%s\n"+
				"[blue]Status:    %s%s\n"+
				"[blue]Action:    %s%s\n"+
				"[blue]Message:   [white]%s\n",
			timeStr, resource, namespace,
			defaultStatusColour, status,
			defaultActionColour, action,
			message,
		)

		detailView := tview.NewTextView()
		detailView.SetDynamicColors(true)
		detailView.SetTextAlign(tview.AlignLeft)
		detailView.SetBorder(true)
		detailView.SetTitle(" Details ")
		detailView.SetBackgroundColor(0x000000)
		detailView.SetText(detail)

		modalFlex := tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(tview.NewBox(), 0, 1, false). // top spacer
			AddItem(
				tview.NewFlex().
					AddItem(tview.NewBox(), 0, 1, false). // left spacer
					AddItem(detailView, 80, 0, true).
					AddItem(tview.NewBox(), 0, 1, false), // right spacer
								15, 0, true).
			AddItem(tview.NewBox(), 0, 1, false) // bottom spacer

		app.SetRoot(modalFlex, true).SetFocus(detailView)

		detailView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEsc || event.Rune() == 'q' {
				app.SetRoot(frame, true).SetFocus(table)
				return nil
			}
			return event
		})
	}
}
