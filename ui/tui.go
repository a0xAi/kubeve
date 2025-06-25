package ui

import (
	"fmt"
	"os"
	"regexp"
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
	var recentNamespaces []string
	var header *Header

	cfg := config.Load()
	bgCol := tcell.Color(0x000000)
	if val, err := strconv.ParseInt(strings.TrimPrefix(cfg.Theme.BackgroundColor, "#"), 16, 32); err == nil {
		bgCol = tcell.Color(val)
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

	versionInfo, verErr := kubeClient.Discovery().ServerVersion()
	if verErr != nil {
		fmt.Fprintf(os.Stderr, "Error fetching server version: %v\n", verErr)
		os.Exit(1)
	}

	app := tview.NewApplication()
	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		screen.Clear()
		return false
	})
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	frame := tview.NewFrame(nil).
		SetBorders(1, 1, 1, 1, 1, 1)
	frame.SetBackgroundColor(bgCol)
	frame.SetPrimitive(flex)

	header = NewHeader(
		clusterName,
		namespace,
		versionInfo.GitVersion,
		recentNamespaces,
		cfg.Flags.DisableLogo,
		bgCol,
	)

	table := NewTable(" [::b][green]Autoscroll ✓ ", bgCol)

	var updateNamespace func(string)

	updateNamespace = func(newNS string) {
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
		table.Clear()
		showNamespaceColumn = namespace == metav1.NamespaceAll
		renderTableHeader(table, ColumnOptions{
			Timestamp: showTimestampColumn,
			Namespace: showNamespaceColumn,
			Status:    showStatusColumn,
			Action:    showActionColumn,
			Resource:  showResourceColumn,
		})

		go kube.WatchEvents(namespace, func(event *corev1.Event) {
			app.QueueUpdateDraw(func() {
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
					matched, _ := regexp.MatchString(filterText, msg)
					if matched &&
						(namespace == metav1.NamespaceAll || event.Namespace == namespace) {
						parts := strings.SplitN(msg, "│", 6)
						if len(parts) == 6 {
							row := table.GetRowCount()
							renderRow(table, row, parts, ColumnOptions{
								Timestamp: showTimestampColumn,
								Namespace: showNamespaceColumn,
								Status:    showStatusColumn,
								Action:    showActionColumn,
								Resource:  showResourceColumn,
							})
							table.ScrollToEnd()
							table.Select(table.GetRowCount()-1, 0)
						}
					}
				}
			})
		})
	}
	filter := NewFilter()

	filterContainer := tview.NewFlex().AddItem(filter, 0, 1, true)
	filterContainer.SetBorderStyle(tcell.StyleDefault.Foreground(0xFF0000))

	filter.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			filterText = filter.GetText()
			table.Clear()
			renderTableHeader(table, ColumnOptions{
				Timestamp: showTimestampColumn,
				Namespace: showNamespaceColumn,
				Status:    showStatusColumn,
				Action:    showActionColumn,
				Resource:  showResourceColumn,
			})
			renderTableContent(table, allEvents, filterText, ColumnOptions{
				Timestamp: showTimestampColumn,
				Namespace: showNamespaceColumn,
				Status:    showStatusColumn,
				Action:    showActionColumn,
				Resource:  showResourceColumn,
			})
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
			if autoScroll {
				table.SetTitle("[::b][green]Autoscroll ✓")
			} else {
				table.SetTitle("[::b][red]Autoscroll ✗")
			}
			return nil
		case event.Key() == tcell.KeyCtrlB:
			table.ScrollToEnd()
			table.Select(table.GetRowCount()-1, 0)
			return nil
		case event.Rune() == '/':
			filter.SetText("")
			app.SetFocus(filter)
			return nil
		case event.Key() == tcell.KeyCtrlN:
			NamespacesModal(app, frame, table, namespaceList, updateNamespace)
			return nil
		case event.Rune() == 'T':
			showTimestampColumn = !showTimestampColumn
			renderTable(table, allEvents, filterText, ColumnOptions{
				Timestamp: showTimestampColumn,
				Namespace: showNamespaceColumn,
				Status:    showStatusColumn,
				Action:    showActionColumn,
				Resource:  showResourceColumn,
			})
			return nil
		case event.Rune() == 'A':
			showActionColumn = !showActionColumn
			renderTable(table, allEvents, filterText, ColumnOptions{
				Timestamp: showTimestampColumn,
				Namespace: showNamespaceColumn,
				Status:    showStatusColumn,
				Action:    showActionColumn,
				Resource:  showResourceColumn,
			})
			return nil
		case event.Rune() == 'S':
			showStatusColumn = !showStatusColumn
			renderTable(table, allEvents, filterText, ColumnOptions{
				Timestamp: showTimestampColumn,
				Namespace: showNamespaceColumn,
				Status:    showStatusColumn,
				Action:    showActionColumn,
				Resource:  showResourceColumn,
			})
			return nil
		case event.Rune() == 'R':
			showResourceColumn = !showResourceColumn
			renderTable(table, allEvents, filterText, ColumnOptions{
				Timestamp: showTimestampColumn,
				Namespace: showNamespaceColumn,
				Status:    showStatusColumn,
				Action:    showActionColumn,
				Resource:  showResourceColumn,
			})
			return nil
		case event.Rune() == 'q', event.Key() == tcell.KeyCtrlC:
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
		if row > 0 && row-1 < len(allEvents) {
			parts := strings.SplitN(allEvents[row-1], "│", 6)
			DetailsModal(app, frame, table, parts)
		}
	})

	updateNamespace(namespace)

	flex.AddItem(header.Flex, 7, 0, false).
		AddItem(table, 0, 1, false).
		AddItem(filterContainer, 1, 0, false)

	app.SetRoot(frame, true)
	app.SetFocus(table)
	if err := app.Run(); err != nil {
		panic(err)
	}
}
