package ui

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
)

// Header exposes the header flex and infoView for live updates.
type Header struct {
	Flex        *tview.Flex
	InfoView    *tview.TextView
	RecentNSBox *tview.TextView
}

// NewHeader builds the top-bar with context info, shortcuts and ASCII logo.
func NewHeader(
	clusterName, namespace, kubeRev string,
	recentNamespaces []string,
	disableLogo bool,
) *Header {
	// Context/info pane
	infoView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	namespaceText := namespace
	if namespace == "" {
		namespaceText = "All namespaces"
	}
	infoView.SetText(fmt.Sprintf(
		"[yellow]Cluster:[-] %s\n"+
			"[yellow]Namespace:[-] %s\n"+
			"[yellow]K8s Rev:[-] %s\n"+
			"[yellow]Kubeve Rev:[-] %s\n",
		clusterName, namespaceText, kubeRev, "0.3.0",
	))

	// Recent namespace shortcuts pane
	recentNs := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	var recentLines []string
	recentLines = append(recentLines, "[blue]<0> [white]All Namespaces")
	for i, ns := range recentNamespaces {
		if i >= 3 {
			break
		}
		recentLines = append(recentLines, fmt.Sprintf("[blue]<%d> [white]%s", i+1, ns))
	}
	recentNs.SetText(strings.Join(recentLines, "\n"))

	// Shortcut keys pane
	shortcuts := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	shortcuts.SetText(ActionShortcuts())

	shortcuts2 := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	shortcuts2.SetText(ColumShortcuts())

	logoView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignRight)
	logoView.SetText(LogoText())

	headerFlex := tview.NewFlex().
		AddItem(infoView, 0, 2, false).
		AddItem(recentNs, 0, 1, false).
		AddItem(shortcuts, 0, 2, false).
		AddItem(shortcuts2, 0, 2, false)
	if !disableLogo {
		headerFlex.AddItem(logoView, 0, 2, false)
	}

	return &Header{
		Flex:        headerFlex,
		InfoView:    infoView,
		RecentNSBox: recentNs,
	}
}

func ActionShortcuts() string {
	items := []struct{ key, desc string }{
		{"</>", "Toggle filter"},
		{"<ctrl+s>", "Toggle autoscroll"},
		{"<ctrl+b>", "Go to last event"},
		{"<ctrl+n>", "Change namespace"},
		{"<↑↓>", "Scroll"},
	}
	var lines []string
	for _, it := range items {
		lines = append(lines, fmt.Sprintf("[blue]%s  [white]%s", it.key, it.desc))
	}
	return strings.Join(lines, "\n")
}

func ColumShortcuts() string {
	items := []struct{ key, desc string }{
		{"<shift+t>", "Toggle timestamp"},
		{"<shift+s>", "Toggle status"},
		{"<shift+a>", "Toggle action"},
		{"<shift+r>", "Toggle resource"},
		{"<shift+g>", "Toggle aggregate"},
	}
	var lines []string
	for _, it := range items {
		lines = append(lines, fmt.Sprintf("[blue]%s\t[white]%s", it.key, it.desc))
	}
	return strings.Join(lines, "\n")
}

func LogoText() string {
	return `__        ___.                      
|  | ____ _\_ |__   [red]_______  __ ____ 
[white]|  |/ /  |  \ __ \[red]_/ __ \  \/ // __ \
[white]|    <|  |  / \_\ \  [red]___/\   /\  ___/
[white]|__|_ \____/|___  /[red]\___  >\_/  \___ >
     [white]\/         \/     [red]\/          \/ `
}
