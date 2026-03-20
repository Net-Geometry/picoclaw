// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package ui

import (
	"fmt"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	tuicfg "github.com/sipeed/picoclaw/cmd/picoclaw-launcher-tui/config"
)

// App is the root TUI application.
type App struct {
	tapp           *tview.Application
	pages          *tview.Pages
	pageStack      []string
	cfg            *tuicfg.TUIConfig
	configPath     string
	pageRefreshFns map[string]func()
	headerModelTV  *tview.TextView
	modalOpen      map[string]bool

	modelCache   map[string][]modelEntry
	modelCacheMu sync.RWMutex
	refreshMu    sync.Mutex
}

// cacheKey returns the map key for a (scheme, user) pair.
func cacheKey(schemeName, userName string) string {
	return fmt.Sprintf("%s/%s", schemeName, userName)
}

// cachedModels returns a defensive copy of the cached model list for a user (may be nil).
func (a *App) cachedModels(schemeName, userName string) []modelEntry {
	a.modelCacheMu.RLock()
	defer a.modelCacheMu.RUnlock()
	entries := a.modelCache[cacheKey(schemeName, userName)]
	return append([]modelEntry(nil), entries...)
}

// refreshModelCache fetches models for every user in the config concurrently.
// Serialized by refreshMu so concurrent calls don't race on the cache map.
// When all fetches complete it calls onDone via QueueUpdateDraw.
func (a *App) refreshModelCache(onDone func()) {
	go func() {
		a.refreshMu.Lock()
		defer a.refreshMu.Unlock()

		users := a.cfg.Provider.Users
		schemes := a.cfg.Provider.Schemes

		schemeURL := make(map[string]string, len(schemes))
		for _, s := range schemes {
			schemeURL[s.Name] = s.BaseURL
		}

		var wg sync.WaitGroup
		for _, u := range users {
			baseURL, ok := schemeURL[u.Scheme]
			if !ok || baseURL == "" {
				continue
			}
			if u.Key == "" {
				a.modelCacheMu.Lock()
				if a.modelCache == nil {
					a.modelCache = make(map[string][]modelEntry)
				}
				a.modelCache[cacheKey(u.Scheme, u.Name)] = nil
				a.modelCacheMu.Unlock()
				continue
			}
			wg.Add(1)
			u := u
			bURL := baseURL
			go func() {
				defer wg.Done()
				entries, err := fetchModels(bURL, u.Key)
				a.modelCacheMu.Lock()
				if a.modelCache == nil {
					a.modelCache = make(map[string][]modelEntry)
				}
				if err != nil || len(entries) == 0 {
					a.modelCache[cacheKey(u.Scheme, u.Name)] = nil
				} else {
					a.modelCache[cacheKey(u.Scheme, u.Name)] = entries
				}
				a.modelCacheMu.Unlock()
			}()
		}
		wg.Wait()

		if onDone != nil {
			a.tapp.QueueUpdateDraw(onDone)
		}
	}()
}

// New creates and wires up the TUI application.
func New(cfg *tuicfg.TUIConfig, configPath string) *App {
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorBlack
	tview.Styles.ContrastBackgroundColor = tcell.ColorTeal
	tview.Styles.MoreContrastBackgroundColor = tcell.ColorLime
	tview.Styles.BorderColor = tcell.ColorDarkCyan
	tview.Styles.TitleColor = tcell.ColorAqua
	tview.Styles.GraphicsColor = tcell.ColorDarkCyan
	tview.Styles.PrimaryTextColor = tcell.ColorWhite
	tview.Styles.SecondaryTextColor = tcell.ColorSilver
	tview.Styles.TertiaryTextColor = tcell.ColorAqua
	tview.Styles.InverseTextColor = tcell.ColorBlack
	tview.Styles.ContrastSecondaryTextColor = tcell.ColorNavy

	a := &App{
		tapp:           tview.NewApplication(),
		pages:          tview.NewPages(),
		pageStack:      []string{},
		cfg:            cfg,
		configPath:     configPath,
		pageRefreshFns: make(map[string]func()),
		modalOpen:      make(map[string]bool),
	}

	a.tapp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			if len(a.modalOpen) > 0 {
				return nil
			}
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
	a.pages.RemovePage(name)
	a.pages.AddPage(name, page, true, false)
	a.pageStack = append(a.pageStack, name)
	a.pages.SwitchToPage(name)
}

func (a *App) goBack() *tcell.EventKey {
	if len(a.pageStack) <= 1 {
		return nil
	}
	popped := a.pageStack[len(a.pageStack)-1]
	a.pageStack = a.pageStack[:len(a.pageStack)-1]
	a.pages.RemovePage(popped)
	prev := a.pageStack[len(a.pageStack)-1]
	if fn, ok := a.pageRefreshFns[prev]; ok {
		fn()
	}
	if prev == "home" && a.headerModelTV != nil {
		a.headerModelTV.SetText(a.cfg.CurrentModelLabel() + "  ")
	}
	a.pages.SwitchToPage(prev)
	return nil
}

func (a *App) showModal(name string, primitive tview.Primitive) {
	a.modalOpen[name] = true
	a.pages.AddPage(name, primitive, true, true)
}

func (a *App) hideModal(name string) {
	delete(a.modalOpen, name)
	a.pages.HidePage(name)
	a.pages.RemovePage(name)
}

func (a *App) save() {
	if err := tuicfg.Save(a.configPath, a.cfg); err != nil {
		a.showError("save failed: " + err.Error())
	}
}

func (a *App) showError(msg string) {
	modal := tview.NewModal().
		SetText("Error: " + msg).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(_ int, _ string) {
			a.hideModal("error")
		})
	modal.SetBackgroundColor(tcell.ColorNavy)
	modal.SetTextColor(tcell.ColorWhite)
	modal.SetButtonBackgroundColor(tcell.ColorDarkCyan)
	modal.SetButtonTextColor(tcell.ColorWhite)
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
	modal.SetBackgroundColor(tcell.ColorNavy)
	modal.SetTextColor(tcell.ColorWhite)
	modal.SetButtonBackgroundColor(tcell.ColorDarkCyan)
	modal.SetButtonTextColor(tcell.ColorWhite)
	a.showModal("confirm-delete", modal)
}

func centeredForm(form *tview.Form, widthPct, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(tview.NewBox(), 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(tview.NewBox(), 0, 1, false).
			AddItem(form, height, 1, true).
			AddItem(tview.NewBox(), 0, 1, false), 0, widthPct, true).
		AddItem(tview.NewBox(), 0, 1, false)
}

func hintBar(text string) *tview.TextView {
	tv := tview.NewTextView().
		SetText(text).
		SetTextAlign(tview.AlignCenter).
		SetTextColor(tcell.ColorAqua)
	tv.SetBackgroundColor(tcell.ColorMidnightBlue)
	return tv
}

func (a *App) buildShell(pageID string, content tview.Primitive, hint string) tview.Primitive {
	var modelTV *tview.TextView
	if pageID == "home" {
		if a.headerModelTV == nil {
			a.headerModelTV = tview.NewTextView()
			a.headerModelTV.SetTextAlign(tview.AlignRight).
				SetTextColor(tcell.ColorYellow).
				SetDynamicColors(true).
				SetBackgroundColor(tcell.ColorBlack)
		}
		modelTV = a.headerModelTV
		modelTV.SetText(a.cfg.CurrentModelLabel() + "  ")
	} else {
		modelTV = tview.NewTextView()
		modelTV.SetBackgroundColor(tcell.ColorBlack)
	}

	headerLeft := tview.NewTextView().
		SetText("  ▓▓ PICOCLAW LAUNCHER ▓▓").
		SetTextColor(tcell.ColorAqua).
		SetBackgroundColor(tcell.ColorBlack)

	header := tview.NewFlex().
		AddItem(headerLeft, 0, 1, false).
		AddItem(modelTV, 0, 1, false)

	sidebar := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false)
	sidebar.SetBackgroundColor(tcell.ColorNavy)

	activeColor := "[lime]▶ "
	inactiveColor := "[gray]  "

	sbText := "\n"
	if pageID == "home" {
		sbText += activeColor + "HOME[-]\n"
	} else {
		sbText += inactiveColor + "HOME[-]\n"
	}
	if pageID == "schemes" {
		sbText += activeColor + "SCHEMES[-]\n"
	} else {
		sbText += inactiveColor + "SCHEMES[-]\n"
	}
	if pageID == "users" {
		sbText += activeColor + "USERS[-]\n"
	} else {
		sbText += inactiveColor + "USERS[-]\n"
	}
	if pageID == "models" {
		sbText += activeColor + "MODELS[-]\n"
	} else {
		sbText += inactiveColor + "MODELS[-]\n"
	}

	sidebar.SetText(sbText)

	footer := hintBar(hint)

	grid := tview.NewGrid().
		SetRows(1, 0, 1).
		SetColumns(16, 0).
		AddItem(header, 0, 0, 1, 2, 0, 0, false).
		AddItem(sidebar, 1, 0, 1, 1, 0, 0, false).
		AddItem(content, 1, 1, 1, 1, 0, 0, true).
		AddItem(footer, 2, 0, 1, 2, 0, 0, false)

	return grid
}
