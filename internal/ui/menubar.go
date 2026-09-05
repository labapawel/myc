package ui

import (
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
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
	categories := []MenuCategory{
		{
			Title: "Lewy",
			Key:   'l',
			Items: []MenuItem{
				{Title: "Odśwież panel", Shortcut: "Ctrl+R", ActionID: "left_refresh"},
				{Title: "Przełącz na lewy", Shortcut: "Tab", ActionID: "left_activate"},
				{IsSeparator: true},
				{Title: "Katalog domowy", Shortcut: "~", ActionID: "left_home"},
				{Title: "Katalog główny", Shortcut: "/", ActionID: "left_root"},
			},
		},
		{
			Title: "Plik",
			Key:   'p',
			Items: []MenuItem{
				{Title: "Podgląd", Shortcut: "F3", ActionID: "view"},
				{Title: "Edycja", Shortcut: "F4", ActionID: "edit"},
				{Title: "Kopiuj", Shortcut: "F5", ActionID: "copy"},
				{Title: "Zmień nazwę / Przenieś", Shortcut: "F6", ActionID: "move"},
				{Title: "Nowy katalog", Shortcut: "F7", ActionID: "mkdir"},
				{Title: "Usuń", Shortcut: "F8", ActionID: "delete"},
				{IsSeparator: true},
				{Title: "Dzielenie pliku...", Shortcut: "", ActionID: "split"},
				{Title: "Łączenie plików...", Shortcut: "", ActionID: "join"},
				{Title: "Koduj (UUE/XXE/MIME)...", Shortcut: "", ActionID: "encode"},
				{Title: "Dekoduj plik...", Shortcut: "", ActionID: "decode"},
				{IsSeparator: true},
				{Title: "Wyjście", Shortcut: "F10", ActionID: "quit"},
			},
		},
		{
			Title: "Polecenie",
			Key:   'c',
			Items: []MenuItem{
				{Title: "Szukaj plików", Shortcut: "Ctrl+F", ActionID: "search"},
				{Title: "Porównaj pliki (Diff)", Shortcut: "", ActionID: "diff"},
				{Title: "Wyszukaj duplikaty", Shortcut: "", ActionID: "duplicates"},
				{Title: "Synchronizuj katalogi", Shortcut: "", ActionID: "sync"},
				{Title: "Masowa zmiana nazw", Shortcut: "", ActionID: "rename"},
				{Title: "Transmisja szeregowa", Shortcut: "", ActionID: "serial"},
			},
		},
		{
			Title: "Opcje",
			Key:   'o',
			Items: []MenuItem{
				{Title: "Układ poziomy/pionowy", Shortcut: "F2", ActionID: "split_toggle"},
				{Title: "Odśwież oba panele", Shortcut: "Ctrl+R", ActionID: "refresh"},
				{IsSeparator: true},
				{Title: "Zaznacz grupę", Shortcut: "+", ActionID: "select_group"},
				{Title: "Odznacz grupę", Shortcut: "-", ActionID: "unselect_group"},
				{Title: "Odwróć zaznaczenie", Shortcut: "*", ActionID: "invert_select"},
				{Title: "Zaznacz wszystko", Shortcut: "Ctrl+A", ActionID: "select_all"},
				{IsSeparator: true},
				{Title: "O programie", Shortcut: "F1", ActionID: "about"},
			},
		},
		{
			Title: "Prawy",
			Key:   'r',
			Items: []MenuItem{
				{Title: "Odśwież panel", Shortcut: "Ctrl+R", ActionID: "right_refresh"},
				{Title: "Przełącz na prawy", Shortcut: "Tab", ActionID: "right_activate"},
				{IsSeparator: true},
				{Title: "Katalog domowy", Shortcut: "~", ActionID: "right_home"},
				{Title: "Katalog główny", Shortcut: "/", ActionID: "right_root"},
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
