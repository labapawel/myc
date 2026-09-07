package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"myc/internal/i18n"
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
	MenuBar         *MenuBar
	Running         bool
	StatusNotice    string
	Lang            string
}

// NewApp initializes the dual panel application with default language (pl).
func NewApp(leftPath, rightPath string) (*App, error) {
	return NewAppWithLang(leftPath, rightPath, "pl")
}

// NewAppWithLang initializes the dual panel application with the specified language.
func NewAppWithLang(leftPath, rightPath, lang string) (*App, error) {
	if lang == "" {
		lang = i18n.DetectLanguage()
	}

	s, err := tcell.NewScreen()
	if err != nil {
		return nil, fmt.Errorf("błąd inicjalizacji terminala: %w", err)
	}
	if err := s.Init(); err != nil {
		return nil, fmt.Errorf("błąd konfiguracji ekranu: %w", err)
	}

	theme := ClassicBlueTheme()

	leftTitle := strings.ToUpper(i18n.T(lang, "menu_left"))
	rightTitle := strings.ToUpper(i18n.T(lang, "menu_right"))

	left, err := NewPanel(leftTitle, leftPath)
	if err != nil {
		s.Fini()
		return nil, err
	}
	right, err := NewPanel(rightTitle, rightPath)
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
		MenuBar:         NewMenuBarWithLang(lang),
		Running:         true,
		Lang:            lang,
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
	DrawKeyBarWithLang(a.Screen, h-1, w, a.Theme, a.Lang)

	// Top Menu Bar (Midnight Commander style: Lewy Plik Polecenie Opcje Prawy)
	if a.MenuBar != nil {
		a.MenuBar.Draw(a.Screen, w, h, a.Theme)
	}

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

	// If top MenuBar is active, route key to MenuBar
	if a.MenuBar != nil && a.MenuBar.Active {
		a.MenuBar.HandleKey(ev, a.handleMenuAction)
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
		if a.MenuBar != nil {
			a.MenuBar.Toggle()
		}

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

func (a *App) handleMenuAction(actionID string) {
	switch actionID {
	case "view":
		a.actionView()
	case "edit":
		a.actionEdit()
	case "copy":
		a.actionCopy()
	case "move":
		a.actionMoveOrRename()
	case "mkdir":
		a.actionMkdir()
	case "delete":
		a.actionDelete()
	case "split":
		a.actionSplitFile()
	case "join":
		a.actionJoinFiles()
	case "encode":
		a.actionEncodeFile()
	case "decode":
		a.actionDecodeFile()
	case "quit":
		a.actionQuit()
	case "search":
		a.actionSearchFiles()
	case "diff":
		a.runCompareTool()
	case "duplicates":
		a.runDuplicatesTool()
	case "sync":
		a.actionSyncDirs()
	case "rename":
		a.runMultiRenameTool()
	case "serial":
		a.actionSerialTransfer()
	case "split_toggle":
		a.HorizontalSplit = !a.HorizontalSplit
	case "refresh":
		a.ActivePanel.Refresh()
		a.InactivePanel().Refresh()
		a.StatusNotice = "Panele odświeżone"
	case "select_group":
		a.actionSelectGroup()
	case "unselect_group":
		a.actionUnselectGroup()
	case "invert_select":
		a.ActivePanel.InvertSelection()
		a.StatusNotice = "Odwrócono zaznaczenie"
	case "select_all":
		a.ActivePanel.SelectAll()
		a.StatusNotice = "Zaznaczono wszystkie elementy"
	case "about":
		a.actionAbout()
	case "language":
		a.actionSelectLanguage()
	case "left_refresh":
		a.LeftPanel.Refresh()
	case "left_activate":
		a.LeftPanel.Active = true
		a.RightPanel.Active = false
		a.ActivePanel = a.LeftPanel
	case "left_home":
		home, _ := os.UserHomeDir()
		if home != "" {
			a.LeftPanel.VFS.SetPath(home)
			a.LeftPanel.Refresh()
		}
	case "left_root":
		a.LeftPanel.VFS.SetPath("/")
		a.LeftPanel.Refresh()
	case "right_refresh":
		a.RightPanel.Refresh()
	case "right_activate":
		a.RightPanel.Active = true
		a.LeftPanel.Active = false
		a.ActivePanel = a.RightPanel
	case "right_home":
		home, _ := os.UserHomeDir()
		if home != "" {
			a.RightPanel.VFS.SetPath(home)
			a.RightPanel.Refresh()
		}
	case "right_root":
		a.RightPanel.VFS.SetPath("/")
		a.RightPanel.Refresh()
	}
}

func (a *App) actionSplitFile() {
	curr := a.ActivePanel.CurrentEntry()
	if curr == nil || curr.IsDir {
		a.StatusNotice = "Wybierz plik do podzielenia"
		return
	}
	prompt := fmt.Sprintf("Dzielenie '%s'. Rozmiar części w bajtach:", curr.Name)
	defaultSize := "1457664"
	a.ActiveDialog = NewInputDialog("DZIELENIE PLIKU", prompt, defaultSize, func(val string) {
		a.ActiveDialog = nil
		chunkSize, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		if err != nil || chunkSize <= 0 {
			a.StatusNotice = "Nieprawidłowy rozmiar części"
			return
		}
		targetDir := a.InactivePanel().VFS.Path()
		parts, err := operations.SplitFile(curr.Path, targetDir, chunkSize, nil)
		if err != nil {
			a.StatusNotice = "Błąd dzielenia: " + err.Error()
		} else {
			a.ActivePanel.Refresh()
			a.InactivePanel().Refresh()
			a.StatusNotice = fmt.Sprintf("Podzielono na %d części do drugiego panelu", len(parts))
		}
	}, func() { a.ActiveDialog = nil })
}

func (a *App) actionJoinFiles() {
	curr := a.ActivePanel.CurrentEntry()
	if curr == nil || curr.IsDir {
		a.StatusNotice = "Wybierz pierwszy plik części (.001) lub plik .crc"
		return
	}
	targetDir := a.InactivePanel().VFS.Path()
	out, err := operations.JoinFiles(curr.Path, targetDir, nil)
	if err != nil {
		a.StatusNotice = "Błąd łączenia: " + err.Error()
	} else {
		a.ActivePanel.Refresh()
		a.InactivePanel().Refresh()
		a.StatusNotice = "Połączono pomyślnie: " + filepath.Base(out)
	}
}

func (a *App) actionEncodeFile() {
	curr := a.ActivePanel.CurrentEntry()
	if curr == nil || curr.IsDir {
		a.StatusNotice = "Wybierz plik do zakodowania"
		return
	}
	menuItems := []string{
		"1. UUE (Unix-to-Unix Encode)",
		"2. XXE (XXEncode)",
		"3. MIME Base64",
	}
	a.ActiveDialog = NewMenuDialog("KODOWANIE PLIKU", menuItems, func(choice string) {
		a.ActiveDialog = nil
		targetDir := a.InactivePanel().VFS.Path()
		srcFile, err := os.Open(curr.Path)
		if err != nil {
			a.StatusNotice = "Błąd odczytu: " + err.Error()
			return
		}
		defer srcFile.Close()

		switch {
		case strings.HasPrefix(choice, "1"):
			dstPath := filepath.Join(targetDir, curr.Name+".uue")
			dstFile, err := os.Create(dstPath)
			if err != nil {
				a.StatusNotice = "Błąd zapisu: " + err.Error()
				return
			}
			defer dstFile.Close()
			err = operations.UUEncode(srcFile, dstFile, curr.Name, 0644)
			if err != nil {
				a.StatusNotice = "Błąd UUE: " + err.Error()
			} else {
				a.StatusNotice = "Zakodowano UUE: " + filepath.Base(dstPath)
			}
		case strings.HasPrefix(choice, "2"):
			dstPath := filepath.Join(targetDir, curr.Name+".xxe")
			dstFile, err := os.Create(dstPath)
			if err != nil {
				a.StatusNotice = "Błąd zapisu: " + err.Error()
				return
			}
			defer dstFile.Close()
			err = operations.XXEncode(srcFile, dstFile, curr.Name, 0644)
			if err != nil {
				a.StatusNotice = "Błąd XXE: " + err.Error()
			} else {
				a.StatusNotice = "Zakodowano XXE: " + filepath.Base(dstPath)
			}
		case strings.HasPrefix(choice, "3"):
			dstPath := filepath.Join(targetDir, curr.Name+".b64")
			dstFile, err := os.Create(dstPath)
			if err != nil {
				a.StatusNotice = "Błąd zapisu: " + err.Error()
				return
			}
			defer dstFile.Close()
			err = operations.MIMEEncode(srcFile, dstFile)
			if err != nil {
				a.StatusNotice = "Błąd MIME: " + err.Error()
			} else {
				a.StatusNotice = "Zakodowano Base64: " + filepath.Base(dstPath)
			}
		}
		a.InactivePanel().Refresh()
	}, func() { a.ActiveDialog = nil })
}

func (a *App) actionDecodeFile() {
	curr := a.ActivePanel.CurrentEntry()
	if curr == nil || curr.IsDir {
		a.StatusNotice = "Wybierz plik do zdekodowania (.uue, .xxe, .b64)"
		return
	}
	targetDir := a.InactivePanel().VFS.Path()
	srcFile, err := os.Open(curr.Path)
	if err != nil {
		a.StatusNotice = "Błąd: " + err.Error()
		return
	}
	defer srcFile.Close()

	lower := strings.ToLower(curr.Name)
	if strings.HasSuffix(lower, ".uue") {
		out, err := operations.UUDecode(srcFile, targetDir)
		if err != nil {
			a.StatusNotice = "Błąd UUDecode: " + err.Error()
		} else {
			a.InactivePanel().Refresh()
			a.StatusNotice = "Zdekodowano: " + filepath.Base(out)
		}
	} else if strings.HasSuffix(lower, ".xxe") {
		out, err := operations.XXDecode(srcFile, targetDir)
		if err != nil {
			a.StatusNotice = "Błąd XXDecode: " + err.Error()
		} else {
			a.InactivePanel().Refresh()
			a.StatusNotice = "Zdekodowano: " + filepath.Base(out)
		}
	} else {
		outPath := filepath.Join(targetDir, strings.TrimSuffix(curr.Name, filepath.Ext(curr.Name)))
		dstFile, err := os.Create(outPath)
		if err != nil {
			a.StatusNotice = "Błąd zapisu: " + err.Error()
			return
		}
		defer dstFile.Close()
		err = operations.MIMEDecode(srcFile, dstFile)
		if err != nil {
			a.StatusNotice = "Błąd dekodowania: " + err.Error()
		} else {
			a.InactivePanel().Refresh()
			a.StatusNotice = "Zdekodowano: " + filepath.Base(outPath)
		}
	}
}

func (a *App) actionSyncDirs() {
	leftDir := a.LeftPanel.VFS.Path()
	rightDir := a.RightPanel.VFS.Path()
	if leftDir == rightDir {
		a.StatusNotice = "Oba panele wskazują ten sam katalog"
		return
	}

	items, err := operations.CompareDirectories(leftDir, rightDir)
	if err != nil {
		a.StatusNotice = "Błąd porównywania katalogów: " + err.Error()
		return
	}

	toCopy := 0
	for _, it := range items {
		if it.Status != operations.StatusIdentical {
			toCopy++
		}
	}

	msg := fmt.Sprintf("Porównano katalogi:\nLewy: %s\nPrawy: %s\n\nZnaleziono %d różnic.\nCzy chcesz zsynchronizować katalogi?",
		filepath.Base(leftDir), filepath.Base(rightDir), toCopy)

	a.ActiveDialog = NewConfirmDialog("SYNCHRONIZACJA KATALOGÓW", msg, func(choice string) {
		a.ActiveDialog = nil
		if choice == "Tak" {
			err := operations.ExecuteSync(items, nil)
			if err != nil {
				a.StatusNotice = "Błąd synchronizacji: " + err.Error()
			} else {
				a.LeftPanel.Refresh()
				a.RightPanel.Refresh()
				a.StatusNotice = fmt.Sprintf("Zsynchronizowano %d elementów", toCopy)
			}
		}
	}, func() { a.ActiveDialog = nil })
}

func (a *App) actionSerialTransfer() {
	a.ActiveDialog = NewMessageDialog("PORT SZEREGOWY (XMODEM / YMODEM)",
		"Obsługa transmisji szeregowej zintegrowana w silniku operations:\n"+
			"- XMODEM (128B checksum & CRC-16)\n"+
			"- YMODEM (1KB STX z metadanymi pliku)\n\n"+
			"Wybierz plik w panelu i podłącz urządzenie szeregowe.",
		func() { a.ActiveDialog = nil })
}

func (a *App) actionSearchFiles() {
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
}

func (a *App) actionSelectGroup() {
	a.ActiveDialog = NewInputDialog("ZAZNACZANIE GRUPY [+]", "Wzorzec plików (np. *.go, *.txt):", "*.*", func(pattern string) {
		a.ActiveDialog = nil
		if pattern != "" {
			cnt := a.ActivePanel.SelectByPattern(pattern)
			a.StatusNotice = fmt.Sprintf("Zaznaczono %d plików", cnt)
		}
	}, func() {
		a.ActiveDialog = nil
	})
}

func (a *App) actionUnselectGroup() {
	a.ActiveDialog = NewInputDialog("ODZNACZANIE GRUPY [-]", "Wzorzec plików do odznaczenia:", "*.*", func(pattern string) {
		a.ActiveDialog = nil
		if pattern != "" {
			cnt := a.ActivePanel.UnselectByPattern(pattern)
			a.StatusNotice = fmt.Sprintf("Odznaczono %d plików", cnt)
		}
	}, func() {
		a.ActiveDialog = nil
	})
}

func (a *App) actionAbout() {
	a.ActiveDialog = NewMessageDialog("O PROGRAMIE",
		"MYC File Manager v1.0.0\n"+
			"Dwupanelowy menedżer plików dla Windows i Linux.\n"+
			"Licencja: MIT | Autor: Paweł Łaba & Społeczność\n"+
			"Wzorowany na Norton Commander, Total Commander i Midnight Commander.",
		func() { a.ActiveDialog = nil })
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

func (a *App) actionSelectLanguage() {
	a.ActiveDialog = NewInputDialog(
		i18n.T(a.Lang, "menu_language"),
		"Kod języka (pl, en, de, fr, es, it, pt, nl, cs, sk, hu, ro, uk, ru...):",
		a.Lang,
		func(code string) {
			code = i18n.Normalize(code)
			if i18n.IsValid(code) {
				a.Lang = code
				a.MenuBar = NewMenuBarWithLang(code)
				a.LeftPanel.ID = strings.ToUpper(i18n.T(code, "menu_left"))
				a.RightPanel.ID = strings.ToUpper(i18n.T(code, "menu_right"))
				a.StatusNotice = i18n.T(code, "lbl_saved")
			} else {
				a.StatusNotice = i18n.T(a.Lang, "lbl_error") + ": nieznany kod języka"
			}
			a.ActiveDialog = nil
			a.Draw()
		},
		func() {
			a.ActiveDialog = nil
			a.Draw()
		},
	)
}
