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
	list.SetBorder(true).SetTitle(" Active Configuration ").SetTitleColor(tcell.ColorAqua).SetBorderColor(tcell.ColorDarkCyan)
	list.SetMainTextColor(tcell.ColorWhite)
	list.SetSecondaryTextColor(tcell.ColorDarkGray)
	list.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorTeal).Foreground(tcell.ColorWhite))
	list.SetSelectedBackgroundColor(tcell.ColorTeal)
	list.SetSelectedTextColor(tcell.ColorWhite)

	rebuildList := func() {
		sel := list.GetCurrentItem()
		list.Clear()
		list.AddItem("model: "+a.cfg.CurrentModelLabel(), "Enter to configure", 'm', func() {
			a.navigateTo("schemes", a.newSchemesPage())
		})
		list.AddItem("Quit", "", 'q', func() { a.tapp.Stop() })
		if sel >= 0 && sel < list.GetItemCount() {
			list.SetCurrentItem(sel)
		}
	}
	rebuildList()

	a.pageRefreshFns["home"] = rebuildList

	return a.buildShell("home", list, " m: configure model  q: quit ")
}
