package ui

import (
	"os/exec"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// CommandBar represents the command line prompt at the bottom of the screen.
type CommandBar struct {
	Text       string
	CursorPos  int
	History    []string
	HistIndex  int
	WorkingDir string
	Active     bool
	LastOutput string
}

// NewCommandBar initializes the CLI bar.
func NewCommandBar() *CommandBar {
	return &CommandBar{
		Text:      "",
		CursorPos: 0,
		History:   []string{},
		HistIndex: -1,
		Active:    false,
	}
}

// Draw renders the command prompt.
func (c *CommandBar) Draw(s tcell.Screen, y, totalWidth int, workingDir string, theme *Theme) {
	prompt := workingDir + "> "
	if runtime.GOOS != "windows" {
		prompt = workingDir + "$ "
	}

	// Truncate prompt if too long
	if len(prompt) > 30 {
		prompt = "..." + prompt[len(prompt)-27:]
	}

	// Fill background
	for col := 0; col < totalWidth; col++ {
		s.SetContent(col, y, ' ', nil, tcell.StyleDefault.Background(theme.PanelBg).Foreground(theme.CommandInputFg))
	}

	// Draw prompt
	promptLen := drawString(s, 0, y, prompt, theme.PanelBg, theme.CommandPromptFg, totalWidth)

	// Draw entered text
	drawString(s, promptLen, y, c.Text, theme.PanelBg, theme.CommandInputFg, totalWidth-promptLen)

	// Show cursor if command bar has text or is active
	if c.Active || len(c.Text) > 0 {
		cursorScreenX := promptLen + c.CursorPos
		if cursorScreenX < totalWidth {
			s.ShowCursor(cursorScreenX, y)
		}
	}
}

// HandleKey handles typing in the command line.
func (c *CommandBar) HandleKey(ev *tcell.EventKey) (executed bool, output string) {
	switch ev.Key() {
	case tcell.KeyRune:
		c.Active = true
		c.Text = c.Text[:c.CursorPos] + string(ev.Rune()) + c.Text[c.CursorPos:]
		c.CursorPos++
		return false, ""

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if c.CursorPos > 0 && len(c.Text) > 0 {
			c.Text = c.Text[:c.CursorPos-1] + c.Text[c.CursorPos:]
			c.CursorPos--
		}
		if len(c.Text) == 0 {
			c.Active = false
		}
		return false, ""

	case tcell.KeyDelete:
		if c.CursorPos < len(c.Text) {
			c.Text = c.Text[:c.CursorPos] + c.Text[c.CursorPos+1:]
		}
		return false, ""

	case tcell.KeyLeft:
		if c.CursorPos > 0 {
			c.CursorPos--
		}
		return false, ""

	case tcell.KeyRight:
		if c.CursorPos < len(c.Text) {
			c.CursorPos++
		}
		return false, ""

	case tcell.KeyHome:
		c.CursorPos = 0
		return false, ""

	case tcell.KeyEnd:
		c.CursorPos = len(c.Text)
		return false, ""

	case tcell.KeyEscape:
		c.Text = ""
		c.CursorPos = 0
		c.Active = false
		return false, ""

	case tcell.KeyEnter:
		if strings.TrimSpace(c.Text) == "" {
			c.Active = false
			return false, ""
		}

		cmdStr := strings.TrimSpace(c.Text)
		c.History = append(c.History, cmdStr)
		c.HistIndex = len(c.History)
		c.Text = ""
		c.CursorPos = 0
		c.Active = false

		// Run command
		out, err := c.runCommand(cmdStr)
		if err != nil {
			return true, "Błąd: " + err.Error()
		}
		return true, out

	case tcell.KeyUp:
		if len(c.History) > 0 && c.HistIndex > 0 {
			c.HistIndex--
			c.Text = c.History[c.HistIndex]
			c.CursorPos = len(c.Text)
		}
		return false, ""

	case tcell.KeyDown:
		if c.HistIndex < len(c.History)-1 {
			c.HistIndex++
			c.Text = c.History[c.HistIndex]
			c.CursorPos = len(c.Text)
		} else {
			c.HistIndex = len(c.History)
			c.Text = ""
			c.CursorPos = 0
			c.Active = false
		}
		return false, ""
	}

	return false, ""
}

func (c *CommandBar) runCommand(cmdStr string) (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/c", cmdStr)
	} else {
		cmd = exec.Command("/bin/sh", "-c", cmdStr)
	}
	if c.WorkingDir != "" {
		cmd.Dir = c.WorkingDir
	}
	bytes, err := cmd.CombinedOutput()
	return string(bytes), err
}
