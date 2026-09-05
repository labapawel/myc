package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

type FunctionKey struct {
	Num   int
	Label string
}

var DefaultFunctionKeys = []FunctionKey{
	{1, "Pomoc"},
	{2, "Układ"},
	{3, "Podgląd"},
	{4, "Edycja"},
	{5, "Kopiuj"},
	{6, "ZmieńN"},
	{7, "NowyKat"},
	{8, "Usuń"},
	{9, "Narzędzia"},
	{10, "Wyjście"},
}

// DrawKeyBar renders the 10 function keys across the bottom line.
func DrawKeyBar(s tcell.Screen, y, totalWidth int, theme *Theme) {
	numKeys := len(DefaultFunctionKeys)
	slotWidth := totalWidth / numKeys
	if slotWidth < 4 {
		slotWidth = 4
	}

	for i, fk := range DefaultFunctionKeys {
		startX := i * slotWidth
		if startX >= totalWidth {
			break
		}

		// Draw number (1..10)
		numStr := fmt.Sprintf("%d", fk.Num)
		numWidth := len(numStr)
		drawString(s, startX, y, numStr, theme.KeyBarNumberBg, theme.KeyBarNumberFg, numWidth)

		// Draw label
		labelWidth := slotWidth - numWidth
		if labelWidth > 0 {
			// Fill remainder with background
			style := tcell.StyleDefault.Background(theme.KeyBarTextBg).Foreground(theme.KeyBarTextFg)
			for c := startX + numWidth; c < startX+slotWidth && c < totalWidth; c++ {
				s.SetContent(c, y, ' ', nil, style)
			}
			drawString(s, startX+numWidth, y, fk.Label, theme.KeyBarTextBg, theme.KeyBarTextFg, labelWidth)
		}
	}
}
