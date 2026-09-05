package ui

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"myc/internal/vfs"
)

// Lister represents the F3 file previewer (Text / Hex).
type Lister struct {
	FilePath string
	FileName string
	FileSize int64
	Content  []byte
	Lines    []string
	HexLines []string
	HexMode  bool
	TopLine  int
	Active   bool
}

// NewLister loads file content from vfs or local file.
func NewLister(entry *vfs.FileEntry, currentVfs vfs.VFS) (*Lister, error) {
	var data []byte
	var err error

	if currentVfs != nil {
		rc, err := currentVfs.Open(entry.Name)
		if err == nil {
			defer rc.Close()
			data, err = io.ReadAll(rc)
		}
	}

	if data == nil {
		data, err = os.ReadFile(entry.Path)
		if err != nil {
			return nil, err
		}
	}

	// Prepare text lines (handle both CRLF and LF)
	raw := string(data)
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	lines := strings.Split(raw, "\n")

	// Prepare hex lines
	hexDump := hex.Dump(data)
	hexDump = strings.ReplaceAll(hexDump, "\r\n", "\n")
	hexLines := strings.Split(hexDump, "\n")

	return &Lister{
		FilePath: entry.Path,
		FileName: entry.Name,
		FileSize: int64(len(data)),
		Content:  data,
		Lines:    lines,
		HexLines: hexLines,
		HexMode:  false,
		TopLine:  0,
		Active:   true,
	}, nil
}

// Draw renders the lister onto screen.
func (l *Lister) Draw(s tcell.Screen, width, height int, theme *Theme) {
	// Header bar
	header := fmt.Sprintf(" PODGLĄD [F3]: %s (%s) | Tryb: %s (H-przełącz) | Esc-Zamknij ",
		l.FileName, FormatSize(l.FileSize, false), map[bool]string{true: "HEX", false: "TEKST"}[l.HexMode])
	drawBar(s, 0, 0, width, header, theme.HeaderBg, theme.HeaderFg)

	activeLines := l.Lines
	if l.HexMode {
		activeLines = l.HexLines
	}

	viewHeight := height - 2
	for row := 0; row < viewHeight; row++ {
		lineIdx := l.TopLine + row
		screenY := row + 1

		// Clear row with background
		for col := 0; col < width; col++ {
			s.SetContent(col, screenY, ' ', nil, tcell.StyleDefault.Background(theme.PanelBg).Foreground(theme.PanelFg))
		}

		if lineIdx < len(activeLines) {
			text := activeLines[lineIdx]
			var lineContent string
			if l.HexMode {
				lineContent = text
			} else {
				lineContent = fmt.Sprintf("%5d | %s", lineIdx+1, text)
			}
			drawString(s, 1, screenY, lineContent, theme.PanelBg, theme.PanelFg, width-2)
		}
	}

	// Footer bar
	footer := fmt.Sprintf(" Linia: %d/%d (%.1f%%) | Strzałki/PgUp/PgDn: Nawigacja | H: Hex | Esc/F3: Wyjście ",
		l.TopLine+1, len(activeLines), float64(l.TopLine+1)/float64(len(activeLines))*100)
	drawBar(s, 0, height-1, width, footer, theme.StatusBarBg, theme.StatusBarFg)
}

// HandleKey processes keyboard input inside lister.
func (l *Lister) HandleKey(ev *tcell.EventKey, height int) bool {
	viewHeight := height - 2
	activeCount := len(l.Lines)
	if l.HexMode {
		activeCount = len(l.HexLines)
	}

	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyF3:
		l.Active = false
		return true
	case tcell.KeyUp:
		if l.TopLine > 0 {
			l.TopLine--
		}
		return true
	case tcell.KeyDown:
		if l.TopLine < activeCount-viewHeight {
			l.TopLine++
		}
		return true
	case tcell.KeyPgUp:
		l.TopLine -= viewHeight
		if l.TopLine < 0 {
			l.TopLine = 0
		}
		return true
	case tcell.KeyPgDn:
		l.TopLine += viewHeight
		if l.TopLine > activeCount-viewHeight {
			l.TopLine = activeCount - viewHeight
		}
		if l.TopLine < 0 {
			l.TopLine = 0
		}
		return true
	case tcell.KeyHome:
		l.TopLine = 0
		return true
	case tcell.KeyEnd:
		l.TopLine = activeCount - viewHeight
		if l.TopLine < 0 {
			l.TopLine = 0
		}
		return true
	case tcell.KeyRune:
		if ev.Rune() == 'h' || ev.Rune() == 'H' {
			l.HexMode = !l.HexMode
			l.TopLine = 0
			return true
		}
	}
	return false
}
