// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	tuicfg "github.com/sipeed/picoclaw/cmd/picoclaw-launcher-tui/config"
)

func (a *App) newSchemesPage() tview.Primitive {
	list := tview.NewList()
	list.SetBorder(true).SetTitle(" Provider Schemes  (a:add  e:edit  d:delete  Enter:users) ")

	rebuild := func() {
		sel := list.GetCurrentItem()
		list.Clear()
		for _, s := range a.cfg.Provider.Schemes {
			name := s.Name
			list.AddItem(
				fmt.Sprintf("%s  ·  %s  [%s]", s.Name, s.BaseURL, s.Type),
				"",
				0,
				func() {
					a.pages.RemovePage("users")
					a.navigateTo("users", a.newUsersPage(name))
				},
			)
		}
		if sel >= 0 && sel < list.GetItemCount() {
			list.SetCurrentItem(sel)
		}
	}
	rebuild()

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'a':
			a.showSchemeForm(nil, func(s tuicfg.Scheme) {
				a.cfg.Provider.Schemes = append(a.cfg.Provider.Schemes, s)
				a.save()
				rebuild()
			})
			return nil
		case 'e':
			idx := list.GetCurrentItem()
			if idx < 0 || idx >= len(a.cfg.Provider.Schemes) {
				return nil
			}
			orig := a.cfg.Provider.Schemes[idx]
			a.showSchemeForm(&orig, func(s tuicfg.Scheme) {
				a.cfg.Provider.Schemes[idx] = s
				a.save()
				rebuild()
			})
			return nil
		case 'd':
			idx := list.GetCurrentItem()
			if idx < 0 || idx >= len(a.cfg.Provider.Schemes) {
				return nil
			}
			name := a.cfg.Provider.Schemes[idx].Name
			a.confirmDelete(fmt.Sprintf("scheme %q", name), func() {
				schemes := a.cfg.Provider.Schemes
				a.cfg.Provider.Schemes = append(schemes[:idx], schemes[idx+1:]...)
				a.save()
				rebuild()
			})
			return nil
		}
		return event
	})

	footer := hintBar(" Enter: users  a: add  e: edit  d: delete  ESC: back ")

	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(footer, 1, 0, false)
}

func (a *App) showSchemeForm(existing *tuicfg.Scheme, onSave func(tuicfg.Scheme)) {
	name := ""
	baseURL := ""
	schemeType := "openai-compatible"
	title := " Add Scheme "

	if existing != nil {
		name = existing.Name
		baseURL = existing.BaseURL
		schemeType = existing.Type
		title = " Edit Scheme "
	}

	typeOptions := []string{"openai-compatible", "anthropic"}
	typeIdx := 0
	for i, t := range typeOptions {
		if t == schemeType {
			typeIdx = i
			break
		}
	}

	form := tview.NewForm()

	var nameField *tview.InputField

	form.
		AddInputField("Name", name, 40, nil, func(text string) { name = text }).
		AddInputField("Base URL", baseURL, 60, nil, func(text string) { baseURL = text }).
		AddDropDown("Type", typeOptions, typeIdx, func(option string, _ int) { schemeType = option }).
		AddButton("Save", func() {
			_ = nameField
			if name == "" {
				a.showError("Name is required")
				return
			}
			if baseURL == "" {
				a.showError("Base URL is required")
				return
			}
			if existing == nil {
				for _, s := range a.cfg.Provider.Schemes {
					if s.Name == name {
						a.showError(fmt.Sprintf("Scheme name %q already exists", name))
						return
					}
				}
			}
			a.hideModal("scheme-form")
			onSave(tuicfg.Scheme{Name: name, BaseURL: baseURL, Type: schemeType})
		}).
		AddButton("Cancel", func() {
			a.hideModal("scheme-form")
		})

	nameField, _ = form.GetFormItemByLabel("Name").(*tview.InputField)

	form.SetBorder(true).SetTitle(title)

	a.showModal("scheme-form", centeredForm(form, 68, 12))
}
