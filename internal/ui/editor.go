package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"myc/internal/vfs"
)

// Editor represents the F4 text editor.
type Editor struct {
	FilePath   string
	FileName   string
	Lines      []string
	CursorX    int
	CursorY    int
	TopLine    int
	Modified   bool
	Active     bool
	StatusMsg  string
}

// NewEditor creates and opens a file in the editor.
func NewEditor(entry *vfs.FileEntry) (*Editor, error) {
	data, err := os.ReadFile(entry.Path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	raw := string(data)
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}

	return &Editor{
		FilePath: entry.Path,
		FileName: entry.Name,
		Lines:    lines,
		CursorX:  0,
		CursorY:  0,
		TopLine:  0,
		Modified: false,
		Active:   true,
	}, nil
}

// Draw renders the editor.
func (e *Editor) Draw(s tcell.Screen, width, height int, theme *Theme) {
	// Top Header bar
	modTag := ""
	if e.Modified {
		modTag = " [ZMODYFIKOWANO*]"
	}
	header := fmt.Sprintf(" EDYTOR [F4]: %s%s | F2/Ctrl+S: Zapisz | Esc: Wyjście ", e.FileName, modTag)
	drawBar(s, 0, 0, width, header, theme.HeaderBg, theme.HeaderFg)

	viewHeight := height - 2
	for row := 0; row < viewHeight; row++ {
		lineIdx := e.TopLine + row
		screenY := row + 1

		for col := 0; col < width; col++ {
			s.SetContent(col, screenY, ' ', nil, tcell.StyleDefault.Background(theme.PanelBg).Foreground(theme.PanelFg))
		}

		if lineIdx < len(e.Lines) {
			text := e.Lines[lineIdx]
			linePrefix := fmt.Sprintf("%4d | ", lineIdx+1)
			drawString(s, 0, screenY, linePrefix, theme.PanelBg, theme.DirFg, 7)
			drawString(s, 7, screenY, text, theme.PanelBg, theme.PanelFg, width-7)
		}
	}

	// Bottom Status bar
	status := fmt.Sprintf(" Linia: %d/%d, Kol: %d | %s",
		e.CursorY+1, len(e.Lines), e.CursorX+1, e.StatusMsg)
	drawBar(s, 0, height-1, width, status, theme.StatusBarBg, theme.StatusBarFg)

	// Set cursor position on screen
	screenCursorY := (e.CursorY - e.TopLine) + 1
	screenCursorX := e.CursorX + 7
	if screenCursorY >= 1 && screenCursorY < height-1 && screenCursorX < width {
		s.ShowCursor(screenCursorX, screenCursorY)
	}
}

// Save writes editor lines back to disk.
func (e *Editor) Save() error {
	content := strings.Join(e.Lines, "\n")
	err := os.WriteFile(e.FilePath, []byte(content), 0644)
	if err != nil {
		e.StatusMsg = "Błąd zapisu: " + err.Error()
		return err
	}
	e.Modified = false
	e.StatusMsg = "Zapisano pomyślnie!"
	return nil
}

// HandleKey processes keyboard input in editor.
func (e *Editor) HandleKey(ev *tcell.EventKey, height int) bool {
	viewHeight := height - 2

	switch ev.Key() {
	case tcell.KeyEscape:
		e.Active = false
		return true

	case tcell.KeyF2, tcell.KeyCtrlS:
		e.Save()
		return true

	case tcell.KeyUp:
		if e.CursorY > 0 {
			e.CursorY--
			if e.CursorX > len(e.Lines[e.CursorY]) {
				e.CursorX = len(e.Lines[e.CursorY])
			}
			if e.CursorY < e.TopLine {
				e.TopLine = e.CursorY
			}
		}
		return true

	case tcell.KeyDown:
		if e.CursorY < len(e.Lines)-1 {
			e.CursorY++
			if e.CursorX > len(e.Lines[e.CursorY]) {
				e.CursorX = len(e.Lines[e.CursorY])
			}
			if e.CursorY >= e.TopLine+viewHeight {
				e.TopLine = e.CursorY - viewHeight + 1
			}
		}
		return true

	case tcell.KeyLeft:
		if e.CursorX > 0 {
			e.CursorX--
		} else if e.CursorY > 0 {
			e.CursorY--
			e.CursorX = len(e.Lines[e.CursorY])
		}
		return true

	case tcell.KeyRight:
		if e.CursorX < len(e.Lines[e.CursorY]) {
			e.CursorX++
		} else if e.CursorY < len(e.Lines)-1 {
			e.CursorY++
			e.CursorX = 0
		}
		return true

	case tcell.KeyEnter:
		current := e.Lines[e.CursorY]
		remainder := current[e.CursorX:]
		e.Lines[e.CursorY] = current[:e.CursorX]
		newLines := make([]string, 0, len(e.Lines)+1)
		newLines = append(newLines, e.Lines[:e.CursorY+1]...)
		newLines = append(newLines, remainder)
		newLines = append(newLines, e.Lines[e.CursorY+1:]...)
		e.Lines = newLines
		e.CursorY++
		e.CursorX = 0
		e.Modified = true
		if e.CursorY >= e.TopLine+viewHeight {
			e.TopLine = e.CursorY - viewHeight + 1
		}
		return true

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if e.CursorX > 0 {
			line := e.Lines[e.CursorY]
			e.Lines[e.CursorY] = line[:e.CursorX-1] + line[e.CursorX:]
			e.CursorX--
			e.Modified = true
		} else if e.CursorY > 0 {
			prevLine := e.Lines[e.CursorY-1]
			currLine := e.Lines[e.CursorY]
			e.CursorX = len(prevLine)
			e.Lines[e.CursorY-1] = prevLine + currLine
			e.Lines = append(e.Lines[:e.CursorY], e.Lines[e.CursorY+1:]...)
			e.CursorY--
			e.Modified = true
		}
		return true

	case tcell.KeyRune:
		line := e.Lines[e.CursorY]
		e.Lines[e.CursorY] = line[:e.CursorX] + string(ev.Rune()) + line[e.CursorX:]
		e.CursorX++
		e.Modified = true
		return true
	}

	return false
}
