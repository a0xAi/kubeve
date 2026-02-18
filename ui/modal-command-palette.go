package ui

import (
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type CommandPaletteCommand struct {
	Name        string
	Aliases     []string
	Description string
	AcceptsArg  bool
	Run         func(arg string) string
}

type CommandPaletteJump struct {
	Label  string
	Detail string
	Search string
	Row    int
}

type commandPaletteResult struct {
	Command *CommandPaletteCommand
	Jump    *CommandPaletteJump
	Score   int
}

func CommandPaletteModal(
	app *tview.Application,
	frame tview.Primitive,
	focus tview.Primitive,
	commands []CommandPaletteCommand,
	jumps []CommandPaletteJump,
	onJump func(row int),
) {
	input := tview.NewInputField().
		SetLabelStyle(tcell.StyleDefault.
			Foreground(tcell.ColorWhite).
			Background(tcell.Color16)).
		SetFieldStyle(tcell.StyleDefault.
			Foreground(tcell.ColorWhite).
			Background(tcell.Color16)).
		SetLabel(": ").
		SetFieldWidth(0)
	input.SetBorder(false)
	results := make([]commandPaletteResult, 0)
	selection := 0
	prevCapture := app.GetInputCapture()

	closePalette := func() {
		app.SetInputCapture(prevCapture)
		app.SetRoot(frame, true).SetFocus(focus)
	}

	updateResults := func() {
		results = buildCommandPaletteResults(input.GetText(), commands, jumps)
		if selection < 0 {
			selection = 0
		}
		if len(results) == 0 {
			selection = 0
			return
		}
		if selection >= len(results) {
			selection = len(results) - 1
		}
	}

	resultText := func(result commandPaletteResult) string {
		if result.Command != nil {
			return "cmd  " + result.Command.Name
		}
		if result.Jump != nil {
			return "jump " + result.Jump.Label
		}
		return ""
	}

	executeSelection := func() {
		raw := strings.TrimSpace(input.GetText())
		if cmd, arg, ok := parsePaletteCommandInput(raw, commands); ok {
			closePalette()
			if cmd.Run != nil {
				cmd.Run(arg)
			}
			return
		}

		if len(results) == 0 {
			closePalette()
			return
		}

		idx := selection
		if idx < 0 || idx >= len(results) {
			closePalette()
			return
		}

		selected := results[idx]
		closePalette()

		if selected.Command != nil && selected.Command.Run != nil {
			arg := ""
			if selected.Command.AcceptsArg && raw != "" && !strings.Contains(raw, " ") {
				arg = raw
			}
			selected.Command.Run(arg)
			return
		}

		if selected.Jump != nil && onJump != nil {
			onJump(selected.Jump.Row)
		}
	}

	input.SetChangedFunc(func(text string) {
		updateResults()
		selection = 0
	})

	overlay := tview.NewBox().SetBackgroundColor(tcell.Color16).SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		listH := height - 1
		updateResults()

		start := 0
		if len(results) > listH {
			if selection < listH {
				start = 0
			} else {
				start = selection - (listH - 1)
			}
		}
		visibleCount := len(results) - start
		if visibleCount > listH {
			visibleCount = listH
		}
		ofs := 0
		if visibleCount < listH {
			ofs = listH - visibleCount
		}

		borderColor := tcell.ColorRed
		inactiveBorder := tcell.ColorBlack

		for i := 0; i < visibleCount; i++ {
			row := start + i
			var borderBg tcell.Color
			if row == selection {
				borderBg = borderColor
			} else {
				borderBg = inactiveBorder
			}
			screen.SetContent(x, y+ofs+i, ' ', nil, tcell.StyleDefault.Background(borderBg))

			fg := tcell.ColorWhite
			if row == selection {
				fg = tcell.ColorYellow
			}
			tview.Print(screen, resultText(results[row]), x+1, y+ofs+i, width-1, tview.AlignLeft, fg)
		}

		if len(results) == 0 {
			tview.Print(screen, "No matches", x+1, y+listH-1, width-1, tview.AlignLeft, tcell.ColorGray)
		}

		input.SetRect(x, y+listH, width, 1)
		input.Draw(screen)
		return x, y, width, height
	})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			closePalette()
			return nil
		case tcell.KeyEnter:
			executeSelection()
			return nil
		case tcell.KeyUp:
			selection--
			updateResults()
			return nil
		case tcell.KeyDown:
			selection++
			updateResults()
			return nil
		default:
			handler := input.InputHandler()
			if handler != nil {
				handler(event, nil)
			}
			return nil
		}
	})

	updateResults()
	app.SetRoot(overlay, true).SetFocus(input)
}

func buildCommandPaletteResults(
	query string,
	commands []CommandPaletteCommand,
	jumps []CommandPaletteJump,
) []commandPaletteResult {
	q := strings.TrimSpace(query)
	results := make([]commandPaletteResult, 0, len(commands)+len(jumps))

	for i := range commands {
		command := &commands[i]
		matchText := command.Name + " " + strings.Join(command.Aliases, " ") + " " + command.Description
		score, ok := fuzzyMatchScore(q, matchText)
		if !ok {
			continue
		}
		results = append(results, commandPaletteResult{
			Command: command,
			Score:   score + 50,
		})
	}

	for i := range jumps {
		jump := &jumps[i]
		search := jump.Search
		if search == "" {
			search = jump.Label + " " + jump.Detail
		}
		score, ok := fuzzyMatchScore(q, search)
		if !ok {
			continue
		}
		results = append(results, commandPaletteResult{
			Jump:  jump,
			Score: score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Command != nil && results[j].Jump != nil {
			return true
		}
		if results[i].Jump != nil && results[j].Command != nil {
			return false
		}
		if results[i].Command != nil && results[j].Command != nil {
			return results[i].Command.Name < results[j].Command.Name
		}
		if results[i].Jump != nil && results[j].Jump != nil {
			return results[i].Jump.Row > results[j].Jump.Row
		}
		return false
	})

	if q == "" {
		limit := 20
		if len(results) > limit {
			return results[:limit]
		}
	}
	return results
}

func parsePaletteCommandInput(
	raw string,
	commands []CommandPaletteCommand,
) (*CommandPaletteCommand, string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, "", false
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return nil, "", false
	}
	commandToken := strings.ToLower(parts[0])
	arg := strings.TrimSpace(strings.TrimPrefix(trimmed, parts[0]))

	for i := range commands {
		command := &commands[i]
		if strings.ToLower(command.Name) == commandToken {
			return command, arg, true
		}
		for _, alias := range command.Aliases {
			if strings.ToLower(alias) == commandToken {
				return command, arg, true
			}
		}
	}
	return nil, "", false
}

func fuzzyMatchScore(query string, target string) (int, bool) {
	q := strings.ToLower(strings.TrimSpace(query))
	t := strings.ToLower(strings.TrimSpace(target))
	if q == "" {
		return 1, true
	}
	if t == "" {
		return 0, false
	}

	if idx := strings.Index(t, q); idx >= 0 {
		containsScore := 1200 - idx*2 - (len(t) - len(q))
		if containsScore < 1 {
			containsScore = 1
		}
		return containsScore, true
	}

	qr := []rune(q)
	tr := []rune(t)
	qi := 0
	score := 0
	streak := 0
	lastMatch := -2

	for i, ch := range tr {
		if qi >= len(qr) {
			break
		}
		if ch != qr[qi] {
			continue
		}

		base := 8
		if i == 0 || tr[i-1] == ' ' || tr[i-1] == '/' || tr[i-1] == '-' || tr[i-1] == '_' {
			base += 10
		}
		if i == lastMatch+1 {
			streak++
			base += 4 + streak
		} else {
			streak = 0
		}
		score += base
		lastMatch = i
		qi++
	}

	if qi != len(qr) {
		return 0, false
	}

	score -= len(tr) - len(qr)
	if score < 1 {
		score = 1
	}
	return score, true
}
