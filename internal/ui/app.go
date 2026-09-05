package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"myc/internal/operations"
)

// App manages the entire TUI application state and drawing loop.
type App struct {
	Screen          tcell.Screen
	Theme           *Theme
	LeftPanel       *Panel
	RightPanel      *Panel
	ActivePanel     *Panel
	HorizontalSplit bool
	CommandBar      *CommandBar
	ActiveDialog    *Dialog
	ActiveLister    *Lister
	ActiveEditor    *Editor
	Running         bool
	StatusNotice    string
}

// NewApp initializes the dual panel application.
func NewApp(leftPath, rightPath string) (*App, error) {
	s, err := tcell.NewScreen()
	if err != nil {
		return nil, fmt.Errorf("błąd inicjalizacji terminala: %w", err)
	}
	if err := s.Init(); err != nil {
		return nil, fmt.Errorf("błąd konfiguracji ekranu: %w", err)
	}

	theme := ClassicBlueTheme()

	left, err := NewPanel("LEWY", leftPath)
	if err != nil {
		s.Fini()
		return nil, err
	}
	right, err := NewPanel("PRAWY", rightPath)
	if err != nil {
		s.Fini()
		return nil, err
	}

	left.Active = true
	right.Active = false

	app := &App{
		Screen:          s,
		Theme:           theme,
		LeftPanel:       left,
		RightPanel:      right,
		ActivePanel:     left,
		HorizontalSplit: false,
		CommandBar:      NewCommandBar(),
		Running:         true,
	}

	return app, nil
}

// InactivePanel returns the opposite panel.
func (a *App) InactivePanel() *Panel {
	if a.ActivePanel == a.LeftPanel {
		return a.RightPanel
	}
	return a.LeftPanel
}

// SwitchPanel toggles the active panel between left and right.
func (a *App) SwitchPanel() {
	if a.ActivePanel == a.LeftPanel {
		a.LeftPanel.Active = false
		a.RightPanel.Active = true
		a.ActivePanel = a.RightPanel
	} else {
		a.RightPanel.Active = false
		a.LeftPanel.Active = true
		a.ActivePanel = a.LeftPanel
	}
}

// Run starts the main event and draw loop.
func (a *App) Run() error {
	defer a.Screen.Fini()

	for a.Running {
		a.Draw()

		ev := a.Screen.PollEvent()
		switch tev := ev.(type) {
		case *tcell.EventResize:
			a.Screen.Sync()

		case *tcell.EventKey:
			a.HandleKey(tev)
		}
	}

	return nil
}

// Draw renders all UI elements.
func (a *App) Draw() {
	a.Screen.Clear()
	a.Screen.HideCursor()

	w, h := a.Screen.Size()
	if w < 20 || h < 10 {
		drawString(a.Screen, 1, 1, "Zbyt mały rozmiar terminala!", a.Theme.PanelBg, a.Theme.PanelFg, w-2)
		a.Screen.Show()
		return
	}

	// 1. If Lister is active, draw it full screen
	if a.ActiveLister != nil && a.ActiveLister.Active {
		a.ActiveLister.Draw(a.Screen, w, h, a.Theme)
		a.Screen.Show()
		return
	}

	// 2. If Editor is active, draw it full screen
	if a.ActiveEditor != nil && a.ActiveEditor.Active {
		a.ActiveEditor.Draw(a.Screen, w, h, a.Theme)
		a.Screen.Show()
		return
	}

	// Top Title Bar
	titleText := " MYC 1.0 (Midnight/Total Commander) | Windows & Linux | Tab: Panele | F1: Pomoc | F10: Wyjście "
	drawBar(a.Screen, 0, 0, w, titleText, a.Theme.HeaderBg, a.Theme.HeaderFg)

	// Panels Area
	panelTop := 1
	panelBottom := h - 4 // leaves 3 lines at bottom for status, cmd, and F-keys

	if !a.HorizontalSplit {
		// Vertical Split (Side-by-side)
		leftW := w / 2
		rightW := w - leftW
		a.drawPanelBox(a.LeftPanel, 0, panelTop, leftW, panelBottom)
		a.drawPanelBox(a.RightPanel, leftW, panelTop, rightW, panelBottom)
	} else {
		// Horizontal Split (Top / Bottom)
		totalH := panelBottom - panelTop
		topH := totalH / 2
		a.drawPanelBox(a.LeftPanel, 0, panelTop, w, panelTop+topH)
		a.drawPanelBox(a.RightPanel, 0, panelTop+topH, w, panelBottom)
	}

	// Info Line for currently focused item
	infoY := h - 3
	curr := a.ActivePanel.CurrentEntry()
	var infoText string
	if curr != nil {
		infoText = fmt.Sprintf(" %s | %s | %s",
			curr.Name, FormatSize(curr.Size, curr.IsDir), curr.ModTime.Format("2006-01-02 15:04:05"))
	}
	if a.StatusNotice != "" {
		infoText = " [ " + a.StatusNotice + " ]"
	}
	drawBar(a.Screen, 0, infoY, w, infoText, a.Theme.StatusBarBg, a.Theme.StatusBarFg)

	// Command Prompt Bar
	a.CommandBar.Draw(a.Screen, h-2, w, a.ActivePanel.VFS.Path(), a.Theme)

	// Function Key Strip
	DrawKeyBar(a.Screen, h-1, w, a.Theme)

	// Active Modal Dialog on top of everything
	if a.ActiveDialog != nil && a.ActiveDialog.Type != DialogNone {
		a.ActiveDialog.Draw(a.Screen, w, h, a.Theme)
	}

	a.Screen.Show()
}

// drawPanelBox renders a single file panel inside given coordinates.
func (a *App) drawPanelBox(p *Panel, x1, y1, w, y2 int) {
	x2 := x1 + w - 1
	title := p.VFS.Path()
	if len(title) > w-6 {
		title = "..." + title[len(title)-(w-9):]
	}

	borderColor := a.Theme.PanelBorder
	if !p.Active {
		borderColor = tcell.ColorGray
	}

	drawBox(a.Screen, x1, y1, x2, y2, a.Theme.PanelBg, borderColor, title, a.Theme.PanelTitle)

	// Header row inside panel
	headerY := y1 + 1
	nameColW := w - 24
	if nameColW < 10 {
		nameColW = 10
	}
	colHeader := fmt.Sprintf(" %-*s │ %10s │ %s", nameColW, "Nazwa", "Rozmiar", "Data")
	drawBar(a.Screen, x1+1, headerY, w-2, colHeader, a.Theme.HeaderBg, a.Theme.HeaderFg)

	// List entries
	listTop := headerY + 1
	listBottom := y2 - 2
	visibleHeight := listBottom - listTop + 1

	for row := 0; row < visibleHeight; row++ {
		entryIdx := p.TopOffset + row
		screenY := listTop + row

		if entryIdx >= len(p.Entries) {
			break
		}

		entry := p.Entries[entryIdx]
		isCursor := p.Active && entryIdx == p.Cursor

		rowBg := a.Theme.PanelBg
		rowFg := a.Theme.PanelFg

		if entry.IsDir {
			rowFg = a.Theme.DirFg
		} else if entry.IsArchive {
			rowFg = a.Theme.ArchiveFg
		} else if entry.Mode&0111 != 0 {
			rowFg = a.Theme.ExecutableFg
		}

		if entry.Selected {
			rowFg = a.Theme.SelectedFg
		}

		if isCursor {
			rowBg = a.Theme.CursorBg
			rowFg = a.Theme.CursorFg
		}

		// Clear entry row with background
		style := tcell.StyleDefault.Background(rowBg).Foreground(rowFg)
		for c := x1 + 1; c < x2; c++ {
			a.Screen.SetContent(c, screenY, ' ', nil, style)
		}

		selTag := " "
		if entry.Selected {
			selTag = "*"
		}

		displayName := entry.Name
		if len(displayName) > nameColW-2 {
			displayName = displayName[:nameColW-5] + "..."
		}

		timeStr := ""
		if !entry.ModTime.IsZero() {
			timeStr = entry.ModTime.Format("02-01-06 15:04")
		}

		line := fmt.Sprintf("%s%-*s %10s  %s",
			selTag, nameColW-1, displayName, FormatSize(entry.Size, entry.IsDir), timeStr)

		drawString(a.Screen, x1+1, screenY, line, rowBg, rowFg, w-2)
	}

	// Panel bottom summary
	summaryY := y2 - 1
	selCount, selBytes := p.SelectionStats()
	summaryText := fmt.Sprintf(" %d el. ", len(p.Entries))
	if selCount > 0 {
		summaryText = fmt.Sprintf(" Zazn.: %d (%s) / %d el. ", selCount, FormatSize(selBytes, false), len(p.Entries))
	}
	drawString(a.Screen, x1+2, summaryY, summaryText, a.Theme.PanelBg, a.Theme.PanelTitle, w-4)
}

// HandleKey handles all key events for panels, dialogs, and commands.
func (a *App) HandleKey(ev *tcell.EventKey) {
	// If modal dialog is open, route key exclusively to dialog
	if a.ActiveDialog != nil && a.ActiveDialog.Type != DialogNone {
		a.ActiveDialog.HandleKey(ev)
		return
	}

	// If Lister is active, route key to lister
	if a.ActiveLister != nil && a.ActiveLister.Active {
		_, h := a.Screen.Size()
		a.ActiveLister.HandleKey(ev, h)
		return
	}

	// If Editor is active, route key to editor
	if a.ActiveEditor != nil && a.ActiveEditor.Active {
		_, h := a.Screen.Size()
		a.ActiveEditor.HandleKey(ev, h)
		return
	}

	// If command bar is active or user typed character with modifier
	if a.CommandBar.Active {
		if ev.Key() == tcell.KeyEscape {
			a.CommandBar.Active = false
			a.CommandBar.Text = ""
			return
		}
		executed, out := a.CommandBar.HandleKey(ev)
		if executed {
			a.ActivePanel.Refresh()
			a.InactivePanel().Refresh()
			if out != "" {
				a.ActiveDialog = NewMessageDialog("WYNIK POLECENIA", out, func() {
					a.ActiveDialog = nil
				})
			}
		}
		return
	}

	_, h := a.Screen.Size()
	panelBottom := h - 4
	panelTop := 1
	visibleHeight := panelBottom - panelTop - 3
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	switch ev.Key() {
	case tcell.KeyTab:
		a.SwitchPanel()

	case tcell.KeyUp:
		a.ActivePanel.MoveUp()

	case tcell.KeyDown:
		a.ActivePanel.MoveDown(visibleHeight)

	case tcell.KeyPgUp:
		a.ActivePanel.PageUp(visibleHeight)

	case tcell.KeyPgDn:
		a.ActivePanel.PageDown(visibleHeight)

	case tcell.KeyHome:
		a.ActivePanel.Home()

	case tcell.KeyEnd:
		a.ActivePanel.End(visibleHeight)

	case tcell.KeyInsert:
		a.ActivePanel.ToggleSelect(visibleHeight)

	case tcell.KeyEnter:
		navigated, err := a.ActivePanel.Enter()
		if err != nil {
			a.StatusNotice = "Błąd: " + err.Error()
		} else if !navigated {
			// If not navigated, it's a file: trigger F3 Lister by default
			a.actionView()
		}

	case tcell.KeyF1:
		a.ActiveDialog = ShowHelpDialog(func() {
			a.ActiveDialog = nil
		})

	case tcell.KeyF2:
		a.HorizontalSplit = !a.HorizontalSplit

	case tcell.KeyF3:
		a.actionView()

	case tcell.KeyF4:
		a.actionEdit()

	case tcell.KeyF5:
		a.actionCopy()

	case tcell.KeyF6:
		a.actionMoveOrRename()

	case tcell.KeyF7:
		a.actionMkdir()

	case tcell.KeyF8, tcell.KeyDelete:
		a.actionDelete()

	case tcell.KeyF9:
		a.actionToolsMenu()

	case tcell.KeyF10:
		a.actionQuit()

	case tcell.KeyCtrlA:
		a.ActivePanel.SelectAll()

	case tcell.KeyCtrlF:
		a.ActiveDialog = NewInputDialog("SZUKAJ PLIKÓW [Ctrl+F]", "Wzorzec nazwy pliku (np. *.go, *.md):", "*", func(pattern string) {
			a.ActiveDialog = nil
			if pattern == "" {
				return
			}
			var matches []operations.SearchMatch
			filter := operations.SearchFilter{NamePattern: pattern}
			err := operations.SearchFiles(a.ActivePanel.VFS.Path(), filter, func(m operations.SearchMatch) {
				matches = append(matches, m)
			}, nil)
			if err != nil {
				a.StatusNotice = "Błąd szukania: " + err.Error()
			} else {
				a.StatusNotice = fmt.Sprintf("Znaleziono %d pasujących plików dla wzorca %s", len(matches), pattern)
			}
		}, func() {
			a.ActiveDialog = nil
		})

	case tcell.KeyCtrlU:
		a.HorizontalSplit = !a.HorizontalSplit

	case tcell.KeyCtrlR:
		a.ActivePanel.Refresh()
		a.InactivePanel().Refresh()
		a.StatusNotice = "Panele odświeżone"

	case tcell.KeyRune:
		switch ev.Rune() {
		case ' ':
			a.ActivePanel.ToggleSelect(visibleHeight)
		case '+':
			a.ActiveDialog = NewInputDialog("ZAZNACZANIE GRUPY [+]", "Wzorzec plików (np. *.go, *.txt):", "*.*", func(pattern string) {
				a.ActiveDialog = nil
				if pattern != "" {
					cnt := a.ActivePanel.SelectByPattern(pattern)
					a.StatusNotice = fmt.Sprintf("Zaznaczono %d plików", cnt)
				}
			}, func() {
				a.ActiveDialog = nil
			})
		case '-':
			a.ActiveDialog = NewInputDialog("ODZNACZANIE GRUPY [-]", "Wzorzec plików do odznaczenia:", "*.*", func(pattern string) {
				a.ActiveDialog = nil
				if pattern != "" {
					cnt := a.ActivePanel.UnselectByPattern(pattern)
					a.StatusNotice = fmt.Sprintf("Odznaczono %d plików", cnt)
				}
			}, func() {
				a.ActiveDialog = nil
			})
		case '*':
			a.ActivePanel.InvertSelection()
			a.StatusNotice = "Odwrócono zaznaczenie"
		default:
			// Start typing in command prompt
			a.CommandBar.Active = true
			a.CommandBar.WorkingDir = a.ActivePanel.VFS.Path()
			a.CommandBar.HandleKey(ev)
		}
	}
}

func (a *App) actionView() {
	curr := a.ActivePanel.CurrentEntry()
	if curr == nil || curr.IsDir {
		return
	}
	lister, err := NewLister(curr, a.ActivePanel.VFS)
	if err != nil {
		a.StatusNotice = "Błąd otwarcia: " + err.Error()
		return
	}
	a.ActiveLister = lister
}

func (a *App) actionEdit() {
	curr := a.ActivePanel.CurrentEntry()
	if curr == nil || curr.IsDir {
		return
	}
	editor, err := NewEditor(curr)
	if err != nil {
		a.StatusNotice = "Błąd edycji: " + err.Error()
		return
	}
	a.ActiveEditor = editor
}

func (a *App) actionCopy() {
	items := a.ActivePanel.GetSelectedOrCurrent()
	if len(items) == 0 {
		return
	}

	targetDir := a.InactivePanel().VFS.Path()
	prompt := fmt.Sprintf("Kopiuj %d element(y/ów) do:", len(items))
	a.ActiveDialog = NewInputDialog("KOPIOWANIE [F5]", prompt, targetDir, func(dst string) {
		a.ActiveDialog = nil
		go func() {
			for _, item := range items {
				dstPath := filepath.Join(dst, item.Name)
				if item.IsDir {
					operations.CopyDir(item.Path, dstPath, nil)
				} else {
					operations.CopyFile(item.Path, dstPath, nil)
				}
			}
			a.ActivePanel.UnselectAll()
			a.ActivePanel.Refresh()
			a.InactivePanel().Refresh()
			a.StatusNotice = fmt.Sprintf("Skopiowano %d element(y/ów)", len(items))
		}()
	}, func() {
		a.ActiveDialog = nil
	})
}

func (a *App) actionMoveOrRename() {
	items := a.ActivePanel.GetSelectedOrCurrent()
	if len(items) == 0 {
		return
	}

	if len(items) == 1 {
		// Single item: rename or move
		curr := items[0]
		prompt := fmt.Sprintf("Zmień nazwę lub przenieś '%s':", curr.Name)
		defaultTarget := filepath.Join(a.InactivePanel().VFS.Path(), curr.Name)
		a.ActiveDialog = NewInputDialog("ZMIANA NAZWY / PRZENIESIENIE [F6]", prompt, defaultTarget, func(dst string) {
			a.ActiveDialog = nil
			err := operations.Move(curr.Path, dst)
			if err != nil {
				a.StatusNotice = "Błąd: " + err.Error()
			} else {
				a.ActivePanel.Refresh()
				a.InactivePanel().Refresh()
				a.StatusNotice = "Przeniesiono: " + curr.Name
			}
		}, func() {
			a.ActiveDialog = nil
		})
	} else {
		// Multiple items: move to target dir
		targetDir := a.InactivePanel().VFS.Path()
		prompt := fmt.Sprintf("Przenieś %d element(y/ów) do:", len(items))
		a.ActiveDialog = NewInputDialog("PRZENOSZENIE [F6]", prompt, targetDir, func(dst string) {
			a.ActiveDialog = nil
			for _, item := range items {
				dstPath := filepath.Join(dst, item.Name)
				operations.Move(item.Path, dstPath)
			}
			a.ActivePanel.UnselectAll()
			a.ActivePanel.Refresh()
			a.InactivePanel().Refresh()
			a.StatusNotice = fmt.Sprintf("Przeniesiono %d elementów", len(items))
		}, func() {
			a.ActiveDialog = nil
		})
	}
}

func (a *App) actionMkdir() {
	a.ActiveDialog = NewInputDialog("NOWY KATALOG [F7]", "Podaj nazwę nowego katalogu:", "", func(name string) {
		a.ActiveDialog = nil
		if strings.TrimSpace(name) == "" {
			return
		}
		err := a.ActivePanel.VFS.Mkdir(name)
		if err != nil {
			a.StatusNotice = "Błąd tworzenia katalogu: " + err.Error()
		} else {
			a.ActivePanel.Refresh()
			a.StatusNotice = "Utworzono katalog: " + name
		}
	}, func() {
		a.ActiveDialog = nil
	})
}

func (a *App) actionDelete() {
	items := a.ActivePanel.GetSelectedOrCurrent()
	if len(items) == 0 {
		return
	}

	msg := fmt.Sprintf("Czy na pewno chcesz usunąć:\n'%s'?", items[0].Name)
	if len(items) > 1 {
		msg = fmt.Sprintf("Czy na pewno chcesz usunąć %d zaznaczonych elementów?", len(items))
	}

	a.ActiveDialog = NewConfirmDialog("USUWANIE [F8]", msg, func(val string) {
		a.ActiveDialog = nil
		for _, item := range items {
			operations.Delete(item.Path)
		}
		a.ActivePanel.UnselectAll()
		a.ActivePanel.Refresh()
		a.StatusNotice = fmt.Sprintf("Usunięto %d element(y/ów)", len(items))
	}, func() {
		a.ActiveDialog = nil
	})
}

func (a *App) actionToolsMenu() {
	menuItems := []string{
		"Narzędzie masowej zmiany nazw (Multi-Rename)",
		"Wyszukiwanie duplikatów plików w katalogu",
		"Porównaj pliki wg zawartości (Diff)",
		"Przełącz podział okna (Pionowy / Poziomy)",
		"Informacje o programie",
	}

	a.ActiveDialog = NewMenuDialog("NARZĘDZIA [F9]", menuItems, func(selected string) {
		a.ActiveDialog = nil
		switch {
		case strings.HasPrefix(selected, "Narzędzie masowej zmiany"):
			a.runMultiRenameTool()
		case strings.HasPrefix(selected, "Wyszukiwanie duplikatów"):
			a.runDuplicatesTool()
		case strings.HasPrefix(selected, "Porównaj pliki"):
			a.runCompareTool()
		case strings.HasPrefix(selected, "Przełącz podział"):
			a.HorizontalSplit = !a.HorizontalSplit
		case strings.HasPrefix(selected, "Informacje"):
			a.ActiveDialog = NewMessageDialog("O PROGRAMIE",
				"MYC File Manager v1.0.0\n"+
					"Wieloplatformowy menedżer plików dla Windows i Linux.\n"+
					"Licencja: MIT | Autor: Paweł Łaba & Społeczność\n"+
					"Inspirowany Norton Commander i Total Commander.",
				func() { a.ActiveDialog = nil })
		}
	}, func() {
		a.ActiveDialog = nil
	})
}

func (a *App) runMultiRenameTool() {
	items := a.ActivePanel.GetSelectedOrCurrent()
	if len(items) == 0 {
		a.StatusNotice = "Brak zaznaczonych plików do zmiany nazw"
		return
	}

	var filePaths []string
	for _, it := range items {
		if !it.IsDir {
			filePaths = append(filePaths, it.Path)
		}
	}

	if len(filePaths) == 0 {
		a.StatusNotice = "Zaznacz pliki regularne (nie katalogi)"
		return
	}

	a.ActiveDialog = NewInputDialog("MASOWA ZMIANA NAZW", "Wzorzec: Znajdź=Zamień (lub np. _=[C]):", "_=[C]", func(val string) {
		a.ActiveDialog = nil
		parts := strings.Split(val, "=")
		find := ""
		repl := ""
		if len(parts) >= 1 {
			find = parts[0]
		}
		if len(parts) >= 2 {
			repl = parts[1]
		}

		rule := operations.RenameRule{
			Find:          find,
			Replace:       repl,
			CounterStart:  1,
			CounterStep:   1,
			CounterDigits: 2,
		}

		pairs := operations.PreviewRename(filePaths, rule)
		err := operations.ExecuteRename(pairs)
		if err != nil {
			a.ActiveDialog = NewMessageDialog("BŁĄD ZMIANY NAZW", err.Error(), func() { a.ActiveDialog = nil })
		} else {
			a.ActivePanel.Refresh()
			a.StatusNotice = fmt.Sprintf("Zmieniono nazwy %d plików", len(pairs))
		}
	}, func() {
		a.ActiveDialog = nil
	})
}

func (a *App) runDuplicatesTool() {
	dir := a.ActivePanel.VFS.Path()
	go func() {
		groups, err := operations.FindDuplicates(dir, nil)
		if err != nil {
			a.ActiveDialog = NewMessageDialog("BŁĄD", err.Error(), func() { a.ActiveDialog = nil })
			return
		}

		if len(groups) == 0 {
			a.ActiveDialog = NewMessageDialog("DUPLIKATY", "Nie znaleziono żadnych duplikatów w katalogu.", func() { a.ActiveDialog = nil })
			return
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("Znaleziono %d grup(y) identycznych plików:\n\n", len(groups)))
		for i, g := range groups {
			if i >= 5 {
				b.WriteString(fmt.Sprintf("... i jeszcze %d innych grup\n", len(groups)-5))
				break
			}
			b.WriteString(fmt.Sprintf("Grupa %d (Rozmiar: %s):\n", i+1, FormatSize(g.Size, false)))
			for _, f := range g.Files {
				b.WriteString(fmt.Sprintf("  - %s\n", filepath.Base(f)))
			}
			b.WriteString("\n")
		}

		a.ActiveDialog = NewMessageDialog("WYNIK SZUKANIA DUPLIKATÓW", b.String(), func() { a.ActiveDialog = nil })
	}()
}

func (a *App) runCompareTool() {
	currLeft := a.LeftPanel.CurrentEntry()
	currRight := a.RightPanel.CurrentEntry()

	if currLeft == nil || currRight == nil || currLeft.IsDir || currRight.IsDir {
		a.ActiveDialog = NewMessageDialog("PORÓWNYWANIE", "Wybierz plik w lewym i plik w prawym panelu.", func() { a.ActiveDialog = nil })
		return
	}

	res, err := operations.CompareFiles(currLeft.Path, currRight.Path)
	if err != nil {
		a.ActiveDialog = NewMessageDialog("BŁĄD PORÓWNANIA", err.Error(), func() { a.ActiveDialog = nil })
		return
	}

	icon := "✓"
	if !res.Identical {
		icon = "✗"
	}
	msg := fmt.Sprintf("%s %s\n\nLewy:  %s (%s)\nPrawy: %s (%s)",
		icon, res.Message, currLeft.Name, FormatSize(currLeft.Size, false), currRight.Name, FormatSize(currRight.Size, false))

	a.ActiveDialog = NewMessageDialog("PORÓWNANIE ZAWARTOŚCI", msg, func() { a.ActiveDialog = nil })
}

func (a *App) actionQuit() {
	a.ActiveDialog = NewConfirmDialog("WYJŚCIE [F10]", "Czy na pewno chcesz zakończyć program myc?", func(val string) {
		a.ActiveDialog = nil
		a.Running = false
	}, func() {
		a.ActiveDialog = nil
	})
}
