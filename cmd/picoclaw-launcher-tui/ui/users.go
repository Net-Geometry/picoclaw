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
	table := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)
	table.SetBorder(true).SetTitle(fmt.Sprintf(" Users · %s ", schemeName))
	table.SetTitleColor(tcell.ColorAqua).SetBorderColor(tcell.ColorDarkCyan)
	table.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorTeal).Foreground(tcell.ColorWhite))

	visibleUsers := func() []tuicfg.User {
		var out []tuicfg.User
		for _, u := range a.cfg.Provider.Users {
			if u.Scheme == schemeName {
				out = append(out, u)
			}
		}
		return out
	}

	findUserGlobalIdx := func(userName string) int {
		for i, u := range a.cfg.Provider.Users {
			if u.Scheme == schemeName && u.Name == userName {
				return i
			}
		}
		return -1
	}

	rowToVisIdx := func(row int) int { return row / 2 }

	selectedUserName := func() string {
		row, _ := table.GetSelection()
		users := visibleUsers()
		visIdx := rowToVisIdx(row)
		if visIdx >= 0 && visIdx < len(users) {
			return users[visIdx].Name
		}
		return ""
	}

	rebuild := func() {
		selName := selectedUserName()
		table.Clear()
		users := visibleUsers()
		for i, u := range users {
			nameRow := i * 2
			detailRow := nameRow + 1

			table.SetCell(nameRow, 0,
				tview.NewTableCell(" "+u.Name).
					SetTextColor(tcell.ColorWhite).
					SetExpansion(1).
					SetSelectable(true),
			)
			table.SetCell(nameRow, 1,
				tview.NewTableCell("").
					SetSelectable(false),
			)

			models := a.cachedModels(schemeName, u.Name)
			var detailText string
			if len(models) > 0 {
				detailText = fmt.Sprintf("  %d models", len(models))
			} else {
				detailText = "  [red]Inactive[-]"
			}
			table.SetCell(detailRow, 0,
				tview.NewTableCell(detailText).
					SetTextColor(tcell.ColorDarkGray).
					SetExpansion(1).
					SetSelectable(false),
			)
			table.SetCell(detailRow, 1,
				tview.NewTableCell(u.Type+"  ").
					SetTextColor(tcell.ColorDarkGray).
					SetAlign(tview.AlignRight).
					SetSelectable(false),
			)
		}
		if selName != "" {
			for i, u := range users {
				if u.Name == selName {
					table.Select(i*2, 0)
					return
				}
			}
		}
		if table.GetRowCount() > 0 {
			table.Select(0, 0)
		}
	}
	rebuild()

	a.refreshModelCache(rebuild)
	a.pageRefreshFns["users"] = func() { a.refreshModelCache(rebuild) }

	table.SetSelectedFunc(func(row, _ int) {
		visIdx := rowToVisIdx(row)
		users := visibleUsers()
		if visIdx < 0 || visIdx >= len(users) {
			return
		}
		uName := users[visIdx].Name
		scheme := a.cfg.Provider.SchemeByName(schemeName)
		if scheme == nil {
			a.showError(fmt.Sprintf("Scheme %q not found", schemeName))
			return
		}
		a.navigateTo("models", a.newModelsPage(schemeName, uName, scheme.BaseURL))
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, _ := table.GetSelection()
		visIdx := rowToVisIdx(row)
		users := visibleUsers()
		switch event.Rune() {
		case 'a':
			a.showUserForm(schemeName, nil, func(u tuicfg.User) {
				a.cfg.Provider.Users = append(a.cfg.Provider.Users, u)
				a.save()
				a.refreshModelCache(rebuild)
			})
			return nil
		case 'e':
			if visIdx < 0 || visIdx >= len(users) {
				return nil
			}
			origName := users[visIdx].Name
			orig := a.cfg.Provider.Users[findUserGlobalIdx(origName)]
			a.showUserForm(schemeName, &orig, func(u tuicfg.User) {
				cfgIdx := findUserGlobalIdx(origName)
				if cfgIdx < 0 {
					a.showError(fmt.Sprintf("User %q no longer exists", origName))
					return
				}
				a.cfg.Provider.Users[cfgIdx] = u
				a.save()
				a.refreshModelCache(func() {
					rebuild()
					for i, usr := range visibleUsers() {
						if usr.Name == u.Name {
							table.Select(i*2, 0)
							break
						}
					}
				})
			})
			return nil
		case 'd':
			if visIdx < 0 || visIdx >= len(users) {
				return nil
			}
			uName := users[visIdx].Name
			a.confirmDelete(fmt.Sprintf("user %q", uName), func() {
				cfgIdx := findUserGlobalIdx(uName)
				if cfgIdx < 0 {
					return
				}
				all := a.cfg.Provider.Users
				a.cfg.Provider.Users = append(all[:cfgIdx], all[cfgIdx+1:]...)
				a.save()
				a.refreshModelCache(rebuild)
			})
			return nil
		}
		return event
	})

	return a.buildShell("users", table, " a: add  e: edit  d: delete  Enter: models  ESC: back ")
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
		AddInputField("Name", name, 32, nil, func(text string) { name = text }).
		AddDropDown("Type", typeOptions, typeIdx, func(option string, _ int) { userType = option }).
		AddPasswordField("Key", key, 32, '*', func(text string) { key = text }).
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

	form.SetBorder(true).SetTitle(title).SetTitleColor(tcell.ColorLime)
	form.SetBorderColor(tcell.ColorDarkCyan)
	form.SetFieldBackgroundColor(tcell.ColorBlack)
	form.SetFieldTextColor(tcell.ColorWhite)
	form.SetLabelColor(tcell.ColorAqua)
	form.SetButtonBackgroundColor(tcell.ColorDarkCyan)
	form.SetButtonTextColor(tcell.ColorWhite)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.hideModal("user-form")
			return nil
		}
		return event
	})

	a.showModal("user-form", centeredForm(form, 6, 13))
}
