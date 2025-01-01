package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var tui_logs_view *tview.TextView
var tui_panel_view *tview.TextView
var tui_app *tview.Application

func tui_write_log(log string) {
	tui_logs_view.SetText(log).ScrollToEnd()
}

func tui_write_panel(log string) {
	tui_panel_view.SetText(log)
}

func tui_init() {
	tui_app = tview.NewApplication()

	tui_logs_view = tview.NewTextView().SetChangedFunc(func() { tui_app.Draw(); tui_logs_view.ScrollToEnd() }).
		SetRegions(true).SetTextAlign(tview.AlignLeft).SetDynamicColors(true).SetScrollable(true)

	tui_logs_view.
		SetBorder(true).
		SetTitle("Logs")

	tui_panel_view = tview.NewTextView().SetChangedFunc(func() { tui_app.Draw() }).
		SetRegions(true).SetTextAlign(tview.AlignLeft)

	tui_panel_view.
		SetBorder(true).
		SetTitle("Orders")

	input_field := tview.NewInputField().SetLabel("Input: ")
	input_box := input_field.
		SetLabel("Input: ").
		SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter {
				go input_handler(input_field.GetText())
				input_field.SetText("")
			}
		}).
		SetFieldBackgroundColor(tview.Styles.PrimitiveBackgroundColor)

	grid := tview.NewGrid().
		SetRows(0, 4).
		SetColumns(0, 0).
		SetBorders(true)

	grid.AddItem(tui_logs_view, 0, 0, 1, 1, 0, 0, false).
		AddItem(tui_panel_view, 0, 1, 1, 1, 0, 0, false).
		AddItem(input_box, 1, 0, 1, 2, 0, 0, true)

	go tui_app.SetRoot(grid, true).SetFocus(grid).Run()
}

func tui_close() {
	tui_app.Stop()
}

func tui_run() {
	tui_app.Run()
}
