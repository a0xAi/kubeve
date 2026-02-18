package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/a0xAi/kubeve/kube"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"k8s.io/client-go/kubernetes"
)

func DetailsModal(
	app *tview.Application,
	frame *tview.Frame,
	table *tview.Table,
	parts []string,
	kubeClient *kubernetes.Clientset,
) {
	if len(parts) != 6 {
		return
	}

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

	baseDetail := fmt.Sprintf(
		"[blue]Time:      [white]%s\n"+
			"[blue]Resource:  [white]%s\n"+
			"[blue]Namespace: [white]%s\n"+
			"[blue]Status:    %s%s\n"+
			"[blue]Action:    %s%s\n"+
			"[blue]Message:   [white]%s\n",
		escapeTViewText(timeStr),
		escapeTViewText(resource),
		escapeTViewText(namespace),
		defaultStatusColour, escapeTViewText(status),
		defaultActionColour, escapeTViewText(action),
		escapeTViewText(message),
	)

	detailView := tview.NewTextView()
	detailView.SetDynamicColors(true)
	detailView.SetTextAlign(tview.AlignLeft)
	detailView.SetBorder(true)
	detailView.SetTitle(" Event Drill-Down ")
	detailView.SetBackgroundColor(0x000000)
	detailView.SetScrollable(true)
	detailView.SetText(baseDetail + "\n[gray]Loading resource drill-down...[white]")

	modalFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(
			tview.NewFlex().
				AddItem(tview.NewBox(), 2, 0, false).
				AddItem(detailView, 0, 1, true).
				AddItem(tview.NewBox(), 2, 0, false),
			0, 1, true,
		).
		AddItem(tview.NewBox(), 1, 0, false)

	app.SetRoot(modalFlex, true).SetFocus(detailView)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	closed := false

	detailView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc || event.Rune() == 'q' {
			closed = true
			cancel()
			app.SetRoot(frame, true).SetFocus(table)
			return nil
		}
		return event
	})

	kind, name, ok := splitResource(resource)
	if !ok || kubeClient == nil {
		detailView.SetText(baseDetail + "\n[yellow]Drill-down unavailable for this row.[white]")
		return
	}

	go func() {
		drilldown := kube.GetResourceDrillDown(ctx, kubeClient, namespace, kind, name)
		text := baseDetail +
			"\n[green]Describe[white]\n" + escapeTViewText(drilldown.Describe) +
			"\n\n[green]Related Resources[white]\n" + escapeTViewText(drilldown.Related) +
			"\n\n[green]Recent Logs[white]\n" + escapeTViewText(drilldown.Logs) +
			"\n\n[gray]Esc/q to close. Use arrow keys to scroll.[white]"
		app.QueueUpdateDraw(func() {
			if closed {
				return
			}
			detailView.SetText(text)
		})
	}()
}

func splitResource(resource string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(resource), "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	kind := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	if kind == "" || name == "" {
		return "", "", false
	}
	return kind, name, true
}

func escapeTViewText(text string) string {
	return strings.ReplaceAll(text, "[", "[[")
}
