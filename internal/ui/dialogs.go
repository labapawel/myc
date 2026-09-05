package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type DialogType int

const (
	DialogNone DialogType = iota
	DialogInput
	DialogConfirm
	DialogMessage
	DialogMenu
)

// Dialog represents any pop-up modal dialog.
type Dialog struct {
	Type        DialogType
	Title       string
	Message     string
	InputText   string
	InputCursor int
	Selected    int
	MenuItems   []string
	OnConfirm   func(val string)
	OnCancel    func()
}

func NewInputDialog(title, prompt, defaultText string, onConfirm func(string), onCancel func()) *Dialog {
	return &Dialog{
		Type:        DialogInput,
		Title:       title,
		Message:     prompt,
		InputText:   defaultText,
		InputCursor: len(defaultText),
		OnConfirm:   onConfirm,
		OnCancel:    onCancel,
	}
}

func NewConfirmDialog(title, message string, onConfirm func(string), onCancel func()) *Dialog {
	return &Dialog{
		Type:      DialogConfirm,
		Title:     title,
		Message:   message,
		Selected:  0, // 0: Tak, 1: Nie
		OnConfirm: onConfirm,
		OnCancel:  onCancel,
	}
}

func NewMessageDialog(title, message string, onDismiss func()) *Dialog {
	return &Dialog{
		Type:      DialogMessage,
		Title:     title,
		Message:   message,
		OnConfirm: func(string) { if onDismiss != nil { onDismiss() } },
		OnCancel:  onDismiss,
	}
}

func NewMenuDialog(title string, items []string, onSelect func(string), onCancel func()) *Dialog {
	return &Dialog{
		Type:      DialogMenu,
		Title:     title,
		MenuItems: items,
		Selected:  0,
		OnConfirm: onSelect,
		OnCancel:  onCancel,
	}
}

// Draw renders the modal dialog in the center of the screen.
func (d *Dialog) Draw(s tcell.Screen, screenW, screenH int, theme *Theme) {
	if d == nil || d.Type == DialogNone {
		return
	}

	boxW := 60
	if boxW > screenW-4 {
		boxW = screenW - 4
	}
	boxH := 10
	if d.Type == DialogMenu {
		boxH = len(d.MenuItems) + 6
	} else if d.Type == DialogMessage {
		lines := strings.Split(d.Message, "\n")
		boxH = len(lines) + 6
		if boxH > screenH-4 {
			boxH = screenH - 4
		}
	}

	x1 := (screenW - boxW) / 2
	y1 := (screenH - boxH) / 2
	x2 := x1 + boxW - 1
	y2 := y1 + boxH - 1

	// Draw box
	drawBox(s, x1, y1, x2, y2, theme.DialogBg, theme.DialogBorder, d.Title, theme.KeyBarNumberFg)

	switch d.Type {
	case DialogInput:
		drawString(s, x1+2, y1+2, d.Message, theme.DialogBg, theme.DialogFg, boxW-4)
		// Input field box
		inputY := y1 + 4
		inputW := boxW - 6
		styleInput := tcell.StyleDefault.Background(tcell.ColorNavy).Foreground(tcell.ColorWhite)
		for c := x1 + 3; c < x1+3+inputW; c++ {
			s.SetContent(c, inputY, ' ', nil, styleInput)
		}
		drawString(s, x1+4, inputY, d.InputText, tcell.ColorNavy, tcell.ColorWhite, inputW-2)
		s.ShowCursor(x1+4+d.InputCursor, inputY)

		// Hint
		hint := "[Enter: Zatwierdź]  [Esc: Anuluj]"
		drawString(s, x1+(boxW-len(hint))/2, y2-1, hint, theme.DialogBg, theme.PanelTitle, boxW-4)

	case DialogConfirm:
		lines := strings.Split(d.Message, "\n")
		for i, l := range lines {
			if y1+2+i < y2-2 {
				drawString(s, x1+3, y1+2+i, l, theme.DialogBg, theme.DialogFg, boxW-6)
			}
		}

		// Buttons: [ Tak ]  [ Nie ]
		btnYes := "[ Tak ]"
		btnNo := "[ Nie ]"
		yesStyle := theme.DialogFg
		noStyle := theme.DialogFg
		yesBg := theme.DialogBg
		noBg := theme.DialogBg

		if d.Selected == 0 {
			yesBg = tcell.ColorTeal
			yesStyle = tcell.ColorBlack
		} else {
			noBg = tcell.ColorTeal
			noStyle = tcell.ColorBlack
		}

		buttonY := y2 - 2
		btnX := x1 + (boxW-20)/2
		drawString(s, btnX, buttonY, btnYes, yesBg, yesStyle, 10)
		drawString(s, btnX+12, buttonY, btnNo, noBg, noStyle, 10)

	case DialogMessage:
		lines := strings.Split(d.Message, "\n")
		for i, l := range lines {
			if y1+2+i < y2-2 {
				drawString(s, x1+3, y1+2+i, l, theme.DialogBg, theme.DialogFg, boxW-6)
			}
		}
		btn := "[ OK (Enter / Esc) ]"
		drawString(s, x1+(boxW-len(btn))/2, y2-2, btn, tcell.ColorTeal, tcell.ColorBlack, len(btn))

	case DialogMenu:
		for i, item := range d.MenuItems {
			itemY := y1 + 2 + i
			itemBg := theme.DialogBg
			itemFg := theme.DialogFg
			if i == d.Selected {
				itemBg = tcell.ColorTeal
				itemFg = tcell.ColorBlack
			}
			label := fmt.Sprintf(" %d. %s", i+1, item)
			// Clear line
			for c := x1 + 2; c < x2-1; c++ {
				s.SetContent(c, itemY, ' ', nil, tcell.StyleDefault.Background(itemBg).Foreground(itemFg))
			}
			drawString(s, x1+3, itemY, label, itemBg, itemFg, boxW-6)
		}
		hint := "[Strzałki: Wybór] [Enter: Zatwierdź] [Esc: Anuluj]"
		drawString(s, x1+(boxW-len(hint))/2, y2-1, hint, theme.DialogBg, theme.PanelTitle, boxW-4)
	}
}

// HandleKey processes keyboard input inside dialog. Returns true if dialog was handled.
func (d *Dialog) HandleKey(ev *tcell.EventKey) bool {
	if d == nil || d.Type == DialogNone {
		return false
	}

	switch d.Type {
	case DialogInput:
		switch ev.Key() {
		case tcell.KeyEscape:
			if d.OnCancel != nil {
				d.OnCancel()
			}
			return true
		case tcell.KeyEnter:
			if d.OnConfirm != nil {
				d.OnConfirm(d.InputText)
			}
			return true
		case tcell.KeyRune:
			d.InputText = d.InputText[:d.InputCursor] + string(ev.Rune()) + d.InputText[d.InputCursor:]
			d.InputCursor++
			return true
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if d.InputCursor > 0 && len(d.InputText) > 0 {
				d.InputText = d.InputText[:d.InputCursor-1] + d.InputText[d.InputCursor:]
				d.InputCursor--
			}
			return true
		case tcell.KeyDelete:
			if d.InputCursor < len(d.InputText) {
				d.InputText = d.InputText[:d.InputCursor] + d.InputText[d.InputCursor+1:]
			}
			return true
		case tcell.KeyLeft:
			if d.InputCursor > 0 {
				d.InputCursor--
			}
			return true
		case tcell.KeyRight:
			if d.InputCursor < len(d.InputText) {
				d.InputCursor++
			}
			return true
		}

	case DialogConfirm:
		switch ev.Key() {
		case tcell.KeyEscape:
			if d.OnCancel != nil {
				d.OnCancel()
			}
			return true
		case tcell.KeyLeft, tcell.KeyRight, tcell.KeyTab:
			d.Selected = 1 - d.Selected
			return true
		case tcell.KeyEnter:
			if d.Selected == 0 {
				if d.OnConfirm != nil {
					d.OnConfirm("tak")
				}
			} else {
				if d.OnCancel != nil {
					d.OnCancel()
				}
			}
			return true
		case tcell.KeyRune:
			r := strings.ToLower(string(ev.Rune()))
			if r == "t" || r == "y" {
				if d.OnConfirm != nil {
					d.OnConfirm("tak")
				}
				return true
			} else if r == "n" {
				if d.OnCancel != nil {
					d.OnCancel()
				}
				return true
			}
		}

	case DialogMessage:
		if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyEnter || ev.Key() == tcell.KeyF3 {
			if d.OnConfirm != nil {
				d.OnConfirm("")
			}
			return true
		}

	case DialogMenu:
		switch ev.Key() {
		case tcell.KeyEscape:
			if d.OnCancel != nil {
				d.OnCancel()
			}
			return true
		case tcell.KeyUp:
			if d.Selected > 0 {
				d.Selected--
			}
			return true
		case tcell.KeyDown:
			if d.Selected < len(d.MenuItems)-1 {
				d.Selected++
			}
			return true
		case tcell.KeyEnter:
			if d.OnConfirm != nil && d.Selected >= 0 && d.Selected < len(d.MenuItems) {
				d.OnConfirm(d.MenuItems[d.Selected])
			}
			return true
		}
	}

	return false
}
