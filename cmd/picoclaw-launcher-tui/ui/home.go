// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) newHomePage() tview.Primitive {
	list := tview.NewList()
	list.SetBorder(true).SetTitle(" picoclaw-launcher-tui ")

	rebuildList := func() {
		sel := list.GetCurrentItem()
		list.Clear()
		list.AddItem("model: "+a.cfg.CurrentModelLabel(), "Enter to configure", 'm', func() {
			a.pages.RemovePage("schemes")
			a.navigateTo("schemes", a.newSchemesPage())
		})
		list.AddItem("Quit", "", 'q', func() { a.tapp.Stop() })
		if sel > 0 && sel < list.GetItemCount() {
			list.SetCurrentItem(sel)
		}
	}
	rebuildList()

	a.homeRefreshFn = rebuildList

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		return event
	})

	footer := hintBar(" Enter: select  q: quit ")

	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(footer, 1, 0, false)
}
