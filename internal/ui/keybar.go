package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"myc/internal/i18n"
)

type FunctionKey struct {
	Num   int
	Label string
}

func GetFunctionKeys(lang string) []FunctionKey {
	return []FunctionKey{
		{1, i18n.T(lang, "f1_help")},
		{2, i18n.T(lang, "f2_layout")},
		{3, i18n.T(lang, "f3_view")},
		{4, i18n.T(lang, "f4_edit")},
		{5, i18n.T(lang, "f5_copy")},
		{6, i18n.T(lang, "f6_move")},
		{7, i18n.T(lang, "f7_mkdir")},
		{8, i18n.T(lang, "f8_delete")},
		{9, i18n.T(lang, "f9_menu")},
		{10, i18n.T(lang, "f10_exit")},
	}
}

// DrawKeyBar renders the 10 function keys across the bottom line.
func DrawKeyBar(s tcell.Screen, y, totalWidth int, theme *Theme) {
	DrawKeyBarWithLang(s, y, totalWidth, theme, "pl")
}

// DrawKeyBarWithLang renders the 10 function keys in the specified language.
func DrawKeyBarWithLang(s tcell.Screen, y, totalWidth int, theme *Theme, lang string) {
	keys := GetFunctionKeys(lang)
	numKeys := len(keys)
	slotWidth := totalWidth / numKeys
	if slotWidth < 4 {
		slotWidth = 4
	}

	for i, fk := range keys {
		startX := i * slotWidth
		if startX >= totalWidth {
			break
		}

		numStr := fmt.Sprintf("%d", fk.Num)
		numWidth := len(numStr)
		drawString(s, startX, y, numStr, theme.KeyBarNumberBg, theme.KeyBarNumberFg, numWidth)

		labelWidth := slotWidth - numWidth
		if labelWidth > 0 {
			style := tcell.StyleDefault.Background(theme.KeyBarTextBg).Foreground(theme.KeyBarTextFg)
			for c := startX + numWidth; c < startX+slotWidth && c < totalWidth; c++ {
				s.SetContent(c, y, ' ', nil, style)
			}
			drawString(s, startX+numWidth, y, fk.Label, theme.KeyBarTextBg, theme.KeyBarTextFg, labelWidth)
		}
	}
}
