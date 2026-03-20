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

func (a *App) newUsersPage(schemeName string) tview.Primitive {
	list := tview.NewList()
	list.SetBorder(true).SetTitle(fmt.Sprintf(" Users for scheme %q  (a:add  e:edit  d:delete  Enter:models) ", schemeName))

	indexInCfg := func(visibleIdx int) int {
		count := 0
		for i, u := range a.cfg.Provider.Users {
			if u.Scheme == schemeName {
				if count == visibleIdx {
					return i
				}
				count++
			}
		}
		return -1
	}

	rebuild := func() {
		sel := list.GetCurrentItem()
		list.Clear()
		for _, u := range a.cfg.Provider.Users {
			if u.Scheme != schemeName {
				continue
			}
			uName := u.Name
			uType := u.Type
			list.AddItem(
				fmt.Sprintf("%s  ·  %s", u.Name, uType),
				"",
				0,
				func() {
					a.pages.RemovePage("models")
					scheme := a.cfg.Provider.SchemeByName(schemeName)
					if scheme == nil {
						a.showError(fmt.Sprintf("Scheme %q not found", schemeName))
						return
					}
					a.navigateTo("models", a.newModelsPage(schemeName, uName, scheme.BaseURL))
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
			a.showUserForm(schemeName, nil, func(u tuicfg.User) {
				a.cfg.Provider.Users = append(a.cfg.Provider.Users, u)
				a.save()
				rebuild()
			})
			return nil
		case 'e':
			visIdx := list.GetCurrentItem()
			cfgIdx := indexInCfg(visIdx)
			if cfgIdx < 0 {
				return nil
			}
			orig := a.cfg.Provider.Users[cfgIdx]
			a.showUserForm(schemeName, &orig, func(u tuicfg.User) {
				a.cfg.Provider.Users[cfgIdx] = u
				a.save()
				rebuild()
			})
			return nil
		case 'd':
			visIdx := list.GetCurrentItem()
			cfgIdx := indexInCfg(visIdx)
			if cfgIdx < 0 {
				return nil
			}
			uName := a.cfg.Provider.Users[cfgIdx].Name
			a.confirmDelete(fmt.Sprintf("user %q", uName), func() {
				users := a.cfg.Provider.Users
				a.cfg.Provider.Users = append(users[:cfgIdx], users[cfgIdx+1:]...)
				a.save()
				rebuild()
			})
			return nil
		}
		return event
	})

	footer := hintBar(" Enter: select model  a: add  e: edit  d: delete  ESC: back ")

	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(footer, 1, 0, false)
}

func (a *App) showUserForm(schemeName string, existing *tuicfg.User, onSave func(tuicfg.User)) {
	name := ""
	userType := "key"
	key := ""
	title := " Add User "

	if existing != nil {
		name = existing.Name
		userType = existing.Type
		key = existing.Key
		title = " Edit User "
	}

	typeOptions := []string{"key", "OAuth"}
	typeIdx := 0
	for i, t := range typeOptions {
		if t == userType {
			typeIdx = i
			break
		}
	}

	form := tview.NewForm()
	form.
		AddInputField("Name", name, 40, nil, func(text string) { name = text }).
		AddDropDown("Type", typeOptions, typeIdx, func(option string, _ int) { userType = option }).
		AddPasswordField("Key", key, 60, '*', func(text string) { key = text }).
		AddButton("Save", func() {
			if name == "" {
				a.showError("Name is required")
				return
			}
			if existing == nil {
				for _, u := range a.cfg.Provider.Users {
					if u.Scheme == schemeName && u.Name == name {
						a.showError(fmt.Sprintf("User name %q already exists for this scheme", name))
						return
					}
				}
			}
			a.hideModal("user-form")
			onSave(tuicfg.User{Name: name, Scheme: schemeName, Type: userType, Key: key})
		}).
		AddButton("Cancel", func() {
			a.hideModal("user-form")
		})

	form.SetBorder(true).SetTitle(title)

	a.showModal("user-form", centeredForm(form, 68, 13))
}
