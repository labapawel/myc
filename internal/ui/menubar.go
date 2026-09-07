package ui

import (
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
	"myc/internal/i18n"
)

// MenuItem represents a single command entry in a dropdown menu.
type MenuItem struct {
	Title       string
	Shortcut    string
	ActionID    string
	IsSeparator bool
}

// MenuCategory represents a top-level menu button (e.g. Lewy, Plik, Polecenie, Opcje, Prawy).
type MenuCategory struct {
	Title      string
	Key        rune
	Items      []MenuItem
	StartX     int
	TotalWidth int
}

// MenuBar manages the top menu bar and dropdown navigation like Midnight Commander.
type MenuBar struct {
	Categories    []MenuCategory
	Active        bool
	ActiveCatIdx  int
	ActiveItemIdx int
}

// NewMenuBar initializes the standard Midnight Commander top menu bar.
func NewMenuBar() *MenuBar {
	return NewMenuBarWithLang("pl")
}

// NewMenuBarWithLang initializes the menu bar localized in the requested language.
func NewMenuBarWithLang(lang string) *MenuBar {
	categories := []MenuCategory{
		{
			Title: i18n.T(lang, "menu_left"),
			Key:   'l',
			Items: []MenuItem{
				{Title: i18n.T(lang, "act_refresh"), Shortcut: "Ctrl+R", ActionID: "left_refresh"},
				{Title: i18n.T(lang, "menu_left"), Shortcut: "Tab", ActionID: "left_activate"},
				{IsSeparator: true},
				{Title: i18n.T(lang, "lbl_home_dir"), Shortcut: "~", ActionID: "left_home"},
				{Title: i18n.T(lang, "lbl_parent_dir"), Shortcut: "/", ActionID: "left_root"},
			},
		},
		{
			Title: i18n.T(lang, "menu_file"),
			Key:   'p',
			Items: []MenuItem{
				{Title: i18n.T(lang, "act_view"), Shortcut: "F3", ActionID: "view"},
				{Title: i18n.T(lang, "act_edit"), Shortcut: "F4", ActionID: "edit"},
				{Title: i18n.T(lang, "act_copy"), Shortcut: "F5", ActionID: "copy"},
				{Title: i18n.T(lang, "act_move"), Shortcut: "F6", ActionID: "move"},
				{Title: i18n.T(lang, "act_mkdir"), Shortcut: "F7", ActionID: "mkdir"},
				{Title: i18n.T(lang, "act_delete"), Shortcut: "F8", ActionID: "delete"},
				{IsSeparator: true},
				{Title: i18n.T(lang, "act_split"), Shortcut: "", ActionID: "split"},
				{Title: i18n.T(lang, "act_join"), Shortcut: "", ActionID: "join"},
				{Title: i18n.T(lang, "act_encode"), Shortcut: "", ActionID: "encode"},
				{Title: i18n.T(lang, "act_decode"), Shortcut: "", ActionID: "decode"},
				{IsSeparator: true},
				{Title: i18n.T(lang, "act_exit"), Shortcut: "F10", ActionID: "quit"},
			},
		},
		{
			Title: i18n.T(lang, "menu_commands"),
			Key:   'c',
			Items: []MenuItem{
				{Title: i18n.T(lang, "act_search"), Shortcut: "Ctrl+F", ActionID: "search"},
				{Title: i18n.T(lang, "act_compare"), Shortcut: "", ActionID: "diff"},
				{Title: i18n.T(lang, "tb_search"), Shortcut: "", ActionID: "duplicates"},
				{Title: i18n.T(lang, "act_sync"), Shortcut: "", ActionID: "sync"},
				{Title: i18n.T(lang, "act_multi_rename"), Shortcut: "", ActionID: "rename"},
				{Title: "Transmisja szeregowa", Shortcut: "", ActionID: "serial"},
			},
		},
		{
			Title: i18n.T(lang, "menu_options"),
			Key:   'o',
			Items: []MenuItem{
				{Title: i18n.T(lang, "f2_layout"), Shortcut: "F2", ActionID: "split_toggle"},
				{Title: i18n.T(lang, "act_refresh"), Shortcut: "Ctrl+R", ActionID: "refresh"},
				{IsSeparator: true},
				{Title: i18n.T(lang, "act_select_group"), Shortcut: "+", ActionID: "select_group"},
				{Title: i18n.T(lang, "act_unselect_group"), Shortcut: "-", ActionID: "unselect_group"},
				{Title: i18n.T(lang, "act_invert_select"), Shortcut: "*", ActionID: "invert_select"},
				{Title: i18n.T(lang, "act_select_all"), Shortcut: "Ctrl+A", ActionID: "select_all"},
				{IsSeparator: true},
				{Title: i18n.T(lang, "menu_language") + "...", Shortcut: "", ActionID: "language"},
				{Title: i18n.T(lang, "act_about"), Shortcut: "F1", ActionID: "about"},
			},
		},
		{
			Title: i18n.T(lang, "menu_right"),
			Key:   'r',
			Items: []MenuItem{
				{Title: i18n.T(lang, "act_refresh"), Shortcut: "Ctrl+R", ActionID: "right_refresh"},
				{Title: i18n.T(lang, "menu_right"), Shortcut: "Tab", ActionID: "right_activate"},
				{IsSeparator: true},
				{Title: i18n.T(lang, "lbl_home_dir"), Shortcut: "~", ActionID: "right_home"},
				{Title: i18n.T(lang, "lbl_parent_dir"), Shortcut: "/", ActionID: "right_root"},
			},
		},
	}

	return &MenuBar{
		Categories:    categories,
		Active:        false,
		ActiveCatIdx:  1, // default to "Plik" when opened
		ActiveItemIdx: 0,
	}
}

// Toggle activates or deactivates the top menu bar.
func (m *MenuBar) Toggle() {
	m.Active = !m.Active
	if m.Active {
		m.ActiveItemIdx = m.firstSelectableItem(m.ActiveCatIdx)
	}
}

func (m *MenuBar) firstSelectableItem(catIdx int) int {
	if catIdx < 0 || catIdx >= len(m.Categories) {
		return 0
	}
	cat := m.Categories[catIdx]
	for i, it := range cat.Items {
		if !it.IsSeparator {
			return i
		}
	}
	return 0
}

// HandleKey processes keyboard input when the menu bar is active.
func (m *MenuBar) HandleKey(ev *tcell.EventKey, onAction func(actionID string)) bool {
	if !m.Active {
		return false
	}

	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyF9:
		m.Active = false
		return true

	case tcell.KeyLeft:
		m.ActiveCatIdx = (m.ActiveCatIdx - 1 + len(m.Categories)) % len(m.Categories)
		m.ActiveItemIdx = m.firstSelectableItem(m.ActiveCatIdx)
		return true

	case tcell.KeyRight:
		m.ActiveCatIdx = (m.ActiveCatIdx + 1) % len(m.Categories)
		m.ActiveItemIdx = m.firstSelectableItem(m.ActiveCatIdx)
		return true

	case tcell.KeyUp:
		cat := m.Categories[m.ActiveCatIdx]
		n := len(cat.Items)
		if n == 0 {
			return true
		}
		for count := 0; count < n; count++ {
			m.ActiveItemIdx = (m.ActiveItemIdx - 1 + n) % n
			if !cat.Items[m.ActiveItemIdx].IsSeparator {
				break
			}
		}
		return true

	case tcell.KeyDown:
		cat := m.Categories[m.ActiveCatIdx]
		n := len(cat.Items)
		if n == 0 {
			return true
		}
		for count := 0; count < n; count++ {
			m.ActiveItemIdx = (m.ActiveItemIdx + 1) % n
			if !cat.Items[m.ActiveItemIdx].IsSeparator {
				break
			}
		}
		return true

	case tcell.KeyEnter:
		cat := m.Categories[m.ActiveCatIdx]
		if m.ActiveItemIdx >= 0 && m.ActiveItemIdx < len(cat.Items) {
			item := cat.Items[m.ActiveItemIdx]
			if !item.IsSeparator && item.ActionID != "" {
				m.Active = false
				onAction(item.ActionID)
				return true
			}
		}
		m.Active = false
		return true

	case tcell.KeyRune:
		r := unicode.ToLower(ev.Rune())

		// First check category shortcuts
		for idx, cat := range m.Categories {
			if unicode.ToLower(cat.Key) == r {
				m.ActiveCatIdx = idx
				m.ActiveItemIdx = m.firstSelectableItem(idx)
				return true
			}
		}
	}

	return true
}

// Draw renders the menu bar on row 0, and if active, the dropdown menu below it.
func (m *MenuBar) Draw(s tcell.Screen, screenW, screenH int, theme *Theme) {
	// 1. Draw top bar background
	barStyle := tcell.StyleDefault.Background(theme.MenuBarBg).Foreground(theme.MenuBarFg)
	for col := 0; col < screenW; col++ {
		s.SetContent(col, 0, ' ', nil, barStyle)
	}

	// Calculate positions of categories
	curX := 1
	for idx := range m.Categories {
		cat := &m.Categories[idx]
		label := "  " + cat.Title + "  "
		cat.StartX = curX
		cat.TotalWidth = uniseg.StringWidth(label)

		isActive := m.Active && idx == m.ActiveCatIdx
		if isActive {
			activeStyle := tcell.StyleDefault.Background(theme.MenuBarActiveBg).Foreground(theme.MenuBarActiveFg)
			for x := curX; x < curX+cat.TotalWidth; x++ {
				s.SetContent(x, 0, ' ', nil, activeStyle)
			}
			drawString(s, curX+1, 0, "["+cat.Title+"]", theme.MenuBarActiveBg, theme.MenuBarActiveFg, cat.TotalWidth)
		} else {
			// Inactive category: highlight first letter with yellow/accent
			drawString(s, curX+2, 0, cat.Title, theme.MenuBarBg, theme.MenuBarFg, cat.TotalWidth)
			// Highlight first letter
			if len(cat.Title) > 0 {
				firstRune := rune(cat.Title[0])
				s.SetContent(curX+2, 0, firstRune, nil, tcell.StyleDefault.Background(theme.MenuBarBg).Foreground(theme.SelectedFg))
			}
		}

		curX += cat.TotalWidth + 1
	}

	// 2. If active, draw dropdown box below active category
	if m.Active && m.ActiveCatIdx >= 0 && m.ActiveCatIdx < len(m.Categories) {
		cat := m.Categories[m.ActiveCatIdx]

		// Find max width needed for items
		maxW := 26
		for _, it := range cat.Items {
			if it.IsSeparator {
				continue
			}
			w := uniseg.StringWidth(it.Title) + uniseg.StringWidth(it.Shortcut) + 6
			if w > maxW {
				maxW = w
			}
		}

		boxW := maxW
		boxH := len(cat.Items) + 2
		boxX := cat.StartX
		if boxX+boxW >= screenW {
			boxX = screenW - boxW - 1
		}
		if boxX < 0 {
			boxX = 0
		}
		boxY := 1

		// Draw border box
		drawBox(s, boxX, boxY, boxX+boxW-1, boxY+boxH-1, theme.DialogBg, theme.DialogBorder, "", theme.DialogFg)

		borderStyle := tcell.StyleDefault.Background(theme.DialogBg).Foreground(theme.DialogBorder)

		// Draw items
		for i, it := range cat.Items {
			itemY := boxY + 1 + i
			if it.IsSeparator {
				s.SetContent(boxX, itemY, '├', nil, borderStyle)
				for x := boxX + 1; x < boxX+boxW-1; x++ {
					s.SetContent(x, itemY, '─', nil, borderStyle)
				}
				s.SetContent(boxX+boxW-1, itemY, '┤', nil, borderStyle)
				continue
			}

			isCurItem := i == m.ActiveItemIdx
			itemBg := theme.DialogBg
			itemFg := theme.DialogFg
			if isCurItem {
				itemBg = theme.CursorBg
				itemFg = theme.CursorFg
				// Fill item background line
				curStyle := tcell.StyleDefault.Background(itemBg).Foreground(itemFg)
				for x := boxX + 1; x < boxX+boxW-1; x++ {
					s.SetContent(x, itemY, ' ', nil, curStyle)
				}
			}

			// Draw title
			drawString(s, boxX+2, itemY, it.Title, itemBg, itemFg, boxW-4)

			// Draw shortcut right-aligned
			if it.Shortcut != "" {
				scW := uniseg.StringWidth(it.Shortcut)
				scX := boxX + boxW - 2 - scW
				drawString(s, scX, itemY, it.Shortcut, itemBg, theme.KeyBarNumberFg, scW)
			}
		}
	}
}
