package ui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/a0xAi/kubeve/config"
	"github.com/a0xAi/kubeve/kube"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func StartUI(version string, overrideNamespace string) {
	var filterText string
	var allEvents []string
	var visibleEvents []string
	var rowToVisibleEvent []int
	var recentNamespaces []string
	var header *Header
	var watchCancel context.CancelFunc
	var watchGeneration int
	var bgCol tcell.Color
	var textCol tcell.Color
	cfg := config.Load()
	if val, err := strconv.ParseInt(strings.TrimPrefix(cfg.Theme.BackgroundColor, "#"), 16, 32); err == nil {
		bgCol = tcell.ColorIsRGB | tcell.ColorValid | tcell.Color(val)
	}

	if val, err := strconv.ParseInt(strings.TrimPrefix(cfg.Theme.TextColor, "#"), 16, 32); err == nil {
		textCol = tcell.ColorIsRGB | tcell.ColorValid | tcell.Color(val)
	}

	namespace, rawConfig, kubeClient, namespaceList, err := kube.Kinit(overrideNamespace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing Kubernetes: %v\n", err)
		os.Exit(1)
	}
	currentContext := rawConfig.CurrentContext
	ctxConfig := rawConfig.Contexts[currentContext]
	clusterName := ctxConfig.Cluster
	showTimestampColumn := true
	autoScroll := true
	showNamespaceColumn := (namespace == metav1.NamespaceAll)
	showStatusColumn := true
	showActionColumn := true
	showResourceColumn := true
	aggregateMode := false
	wrapMessages := false
	filterVisible := false

	versionInfo, verErr := kubeClient.Discovery().ServerVersion()
	if verErr != nil {
		fmt.Fprintf(os.Stderr, "Error fetching server version: %v\n", verErr)
		os.Exit(1)
	}

	app := tview.NewApplication()
	tview.Styles.PrimitiveBackgroundColor = bgCol
	tview.Styles.ContrastBackgroundColor = bgCol
	tview.Styles.PrimaryTextColor = textCol

	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		screen.Clear()
		return false
	})
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	frame := tview.NewFrame(nil).
		SetBorders(1, 1, 1, 1, 1, 1)

	frame.SetPrimitive(flex)

	header = NewHeader(
		clusterName,
		namespace,
		versionInfo.GitVersion,
		recentNamespaces,
		cfg.Flags.DisableLogo,
	)

	table := NewTable(" [::b][green]Autoscroll ✓ ")

	currentColumns := func() ColumnOptions {
		return ColumnOptions{
			Timestamp: showTimestampColumn,
			Namespace: showNamespaceColumn,
			Status:    showStatusColumn,
			Action:    showActionColumn,
			Resource:  showResourceColumn,
			Aggregate: aggregateMode,
		}
	}

	updateTableTitle := func() {
		filterTableText := ""
		if filterText != "" {
			filterTableText = "[yellow] [Filter: " + filterText + "]"
		}
		aggregateTableText := "[gray]Raw"
		if aggregateMode {
			aggregateTableText = "[cyan]Aggregate"
		}
		wrapTableText := "[gray]No Wrap"
		if wrapMessages {
			wrapTableText = "[cyan]Wrap"
		}
		if autoScroll {
			table.SetTitle("[::b]" + filterTableText + "[green]Autoscroll ✓ " + aggregateTableText + " " + wrapTableText)
		} else {
			table.SetTitle("[::b]" + filterTableText + "[red]Autoscroll ✗ " + aggregateTableText + " " + wrapTableText)
		}
	}

	refreshTable := func() {
		displayEvents := allEvents
		if aggregateMode {
			displayEvents = aggregateEvents(allEvents)
		}
		visibleEvents = filterEvents(displayEvents, filterText)
		_, _, tableWidth, _ := table.GetInnerRect()
		rowToVisibleEvent = renderTable(table, visibleEvents, "", currentColumns(), wrapMessages, tableWidth)
	}

	var updateNamespace func(string)

	updateNamespace = func(newNS string) {
		if watchCancel != nil {
			watchCancel()
		}
		watchGeneration++
		currentWatchGeneration := watchGeneration

		if newNS == "" {
			namespace = metav1.NamespaceAll
		} else {
			namespace = newNS
		}
		// Update recent namespaces list (no duplicates, max 3)
		if newNS != "" {
			// remove if already present
			for i, ns := range recentNamespaces {
				if ns == newNS {
					recentNamespaces = append(recentNamespaces[:i], recentNamespaces[i+1:]...)
					break
				}
			}
			recentNamespaces = append([]string{newNS}, recentNamespaces...)
			if len(recentNamespaces) > 3 {
				recentNamespaces = recentNamespaces[:3]
			}
		}
		// Refresh RecentNSBox in header
		var recentLines []string
		recentLines = append(recentLines, "[blue]<0> [white]All Namespaces")
		for i, ns := range recentNamespaces {
			recentLines = append(recentLines, fmt.Sprintf("[blue]<%d> [white]%s", i+1, ns))
		}
		header.RecentNSBox.SetText(strings.Join(recentLines, "\n"))
		namespaceText := namespace
		if namespace == "" {
			namespaceText = "All namespaces"
		}
		header.InfoView.SetText(fmt.Sprintf(
			"[yellow]Cluster:[-] %s\n"+
				"[yellow]Namespace:[-] %s\n"+
				"[yellow]K8s Rev:[-] %s\n"+
				"[yellow]Kubeve Rev:[-] %s\n",
			clusterName, namespaceText, versionInfo.GitVersion, version,
		))
		allEvents = nil
		visibleEvents = nil
		rowToVisibleEvent = nil
		showNamespaceColumn = namespace == metav1.NamespaceAll
		refreshTable()

		watchCtx, cancel := context.WithCancel(context.Background())
		watchCancel = cancel

		go func(ns string, generation int) {
			err := kube.WatchEvents(watchCtx, ns, func(event *corev1.Event) {
				app.QueueUpdateDraw(func() {
					if generation != watchGeneration {
						return
					}

					resource := fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name)
					msg := fmt.Sprintf("%-25s │ %-60s │ %-10s │ %-20s │ %-10s │ %s\n",
						event.LastTimestamp.Time.Format(time.RFC3339),
						resource,
						event.Type,
						event.Reason,
						event.Namespace,
						event.Message,
					)

					if autoScroll {
						allEvents = append(allEvents, msg)
						if aggregateMode || wrapMessages {
							refreshTable()
							if aggregateMode && table.GetRowCount() > 1 {
								table.ScrollToBeginning()
								table.Select(1, 0)
							} else if table.GetRowCount() > 1 {
								table.ScrollToEnd()
								table.Select(table.GetRowCount()-1, 0)
							}
						} else {
							if matchesFilter(msg, filterText) &&
								(namespace == metav1.NamespaceAll || event.Namespace == namespace) {
								visibleEvents = append(visibleEvents, msg)
								parts := strings.SplitN(msg, "│", 6)
								if len(parts) == 6 {
									row := table.GetRowCount()
									renderRow(table, row, parts, currentColumns())
									rowToVisibleEvent = append(rowToVisibleEvent, len(visibleEvents)-1)
									table.ScrollToEnd()
									table.Select(table.GetRowCount()-1, 0)
								}
							}
						}
					}
				})
			})
			if err != nil {
				app.QueueUpdateDraw(func() {
					if generation != watchGeneration {
						return
					}
					updateTableTitle()
					table.SetTitle(fmt.Sprintf("%s [red](watch error: %v)", table.GetTitle(), err))
				})
			}
		}(namespace, currentWatchGeneration)
	}
	filter := NewFilter()

	filterContainer := tview.NewFlex().AddItem(filter, 0, 1, true)
	filterContainer.SetBorder(true)
	filterContainer.SetTitle("Filter").SetTitleAlign(tview.AlignLeft)

	filter.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			filterText = filter.GetText()
			updateTableTitle()
			refreshTable()
			flex.ResizeItem(filterContainer, 0, 0)
			filterVisible = false
			app.SetFocus(table)
		}
	})

	handleInput := func(event *tcell.EventKey) *tcell.EventKey {
		// If filter is focused, let normal typing work and ignore shortcuts.
		if app.GetFocus() == filter {
			return event
		}
		switch {
		case event.Key() == tcell.KeyCtrlS:
			autoScroll = !autoScroll
			filterText = filter.GetText()
			updateTableTitle()
			return nil
		case event.Key() == tcell.KeyCtrlB:
			table.ScrollToEnd()
			table.Select(table.GetRowCount()-1, 0)
			return nil
		case event.Rune() == '/':
			if filterVisible {
				flex.ResizeItem(filterContainer, 0, 0)
				filterVisible = false
				app.SetFocus(table)
			} else {
				flex.ResizeItem(filterContainer, 3, 0)
				filterVisible = true
				filter.SetText("")
				app.SetFocus(filter)
			}
			return nil
		case event.Key() == tcell.KeyCtrlN:
			NamespacesModal(app, frame, table, namespaceList, updateNamespace)
			return nil
		case event.Rune() == 'T':
			showTimestampColumn = !showTimestampColumn
			refreshTable()
			return nil
		case event.Rune() == 'A':
			showActionColumn = !showActionColumn
			refreshTable()
			return nil
		case event.Rune() == 'S':
			showStatusColumn = !showStatusColumn
			refreshTable()
			return nil
		case event.Rune() == 'R':
			showResourceColumn = !showResourceColumn
			refreshTable()
			return nil
		case event.Rune() == 'G':
			aggregateMode = !aggregateMode
			updateTableTitle()
			refreshTable()
			if aggregateMode && table.GetRowCount() > 1 {
				table.ScrollToBeginning()
				table.Select(1, 0)
			}
			return nil
		case event.Rune() == 'w':
			wrapMessages = !wrapMessages
			updateTableTitle()
			refreshTable()
			if table.GetRowCount() > 1 {
				table.ScrollToEnd()
				table.Select(table.GetRowCount()-1, 0)
			}
			return nil
		case event.Rune() == 'q', event.Key() == tcell.KeyCtrlC:
			if watchCancel != nil {
				watchCancel()
			}
			app.Stop()
			return nil
		default:
			if event.Rune() >= '0' && event.Rune() <= '3' {
				switch event.Rune() {
				case '0':
					updateNamespace("")
				default:
					idx := int(event.Rune() - '1')
					if idx >= 0 && idx < len(recentNamespaces) {
						updateNamespace(recentNamespaces[idx])
					}
				}
				return nil
			}
			return event
		}
	}

	app.SetInputCapture(handleInput)
	table.SetSelectedFunc(func(row int, column int) {
		if row <= 0 || row-1 >= len(rowToVisibleEvent) {
			return
		}
		idx := rowToVisibleEvent[row-1]
		if idx >= 0 && idx < len(visibleEvents) {
			parts := strings.SplitN(visibleEvents[idx], "│", 6)
			DetailsModal(app, frame, table, parts)
		}
	})

	updateTableTitle()
	updateNamespace(namespace)

	flex.AddItem(header.Flex, 7, 0, false).
		AddItem(table, 0, 1, false).
		AddItem(filterContainer, 0, 0, false)

	app.SetRoot(frame, true)
	app.SetFocus(table)
	if err := app.Run(); err != nil {
		if watchCancel != nil {
			watchCancel()
		}
		panic(err)
	}
	if watchCancel != nil {
		watchCancel()
	}
}
