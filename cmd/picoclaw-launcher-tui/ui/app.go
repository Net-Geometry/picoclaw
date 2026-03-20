// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	tuicfg "github.com/sipeed/picoclaw/cmd/picoclaw-launcher-tui/config"
)

// App is the root TUI application.
type App struct {
	tapp          *tview.Application
	pages         *tview.Pages
	pageStack     []string
	cfg           *tuicfg.TUIConfig
	configPath    string
	homeRefreshFn func()
}

// New creates and wires up the TUI application.
func New(cfg *tuicfg.TUIConfig, configPath string) *App {
	a := &App{
		tapp:       tview.NewApplication(),
		pages:      tview.NewPages(),
		pageStack:  []string{},
		cfg:        cfg,
		configPath: configPath,
	}

	a.tapp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			return a.goBack()
		}
		return event
	})

	a.buildPages()
	return a
}

// Run starts the TUI event loop.
func (a *App) Run() error {
	return a.tapp.SetRoot(a.pages, true).EnableMouse(true).Run()
}

func (a *App) buildPages() {
	a.pages.AddPage("home", a.newHomePage(), true, true)
	a.pageStack = []string{"home"}
}

func (a *App) navigateTo(name string, page tview.Primitive) {
	a.pages.AddPage(name, page, true, false)
	a.pageStack = append(a.pageStack, name)
	a.pages.SwitchToPage(name)
}

func (a *App) goBack() *tcell.EventKey {
	if len(a.pageStack) <= 1 {
		return nil
	}
	a.pageStack = a.pageStack[:len(a.pageStack)-1]
	prev := a.pageStack[len(a.pageStack)-1]
	if prev == "home" && a.homeRefreshFn != nil {
		a.homeRefreshFn()
	}
	a.pages.SwitchToPage(prev)
	return nil
}

func (a *App) showModal(name string, primitive tview.Primitive) {
	a.pages.AddPage(name, primitive, true, true)
}

func (a *App) hideModal(name string) {
	a.pages.HidePage(name)
	a.pages.RemovePage(name)
}

func (a *App) save() {
	_ = tuicfg.Save(a.configPath, a.cfg)
}

func (a *App) showError(msg string) {
	modal := tview.NewModal().
		SetText("Error: " + msg).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(_ int, _ string) {
			a.hideModal("error")
		})
	a.showModal("error", modal)
}

func (a *App) confirmDelete(label string, onConfirm func()) {
	modal := tview.NewModal().
		SetText("Delete " + label + "?\nThis cannot be undone.").
		AddButtons([]string{"Delete", "Cancel"}).
		SetDoneFunc(func(_ int, buttonLabel string) {
			a.hideModal("confirm-delete")
			if buttonLabel == "Delete" {
				onConfirm()
			}
		})
	a.showModal("confirm-delete", modal)
}

func centeredForm(form *tview.Form, width, height int) tview.Primitive {
	return tview.NewGrid().
		SetColumns(0, width, 0).
		SetRows(0, height, 0).
		AddItem(form, 1, 1, 1, 1, 0, 0, true)
}

func hintBar(text string) *tview.TextView {
	tv := tview.NewTextView().
		SetText(text).
		SetTextAlign(tview.AlignCenter)
	tv.SetBackgroundColor(tcell.ColorDarkBlue)
	return tv
}
