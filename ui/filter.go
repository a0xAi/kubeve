package ui

import (
	"github.com/rivo/tview"
)

func NewFilter() *tview.InputField {
	filter := tview.NewInputField()
	return filter
}
