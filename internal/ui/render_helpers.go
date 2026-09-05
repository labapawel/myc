package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

// drawString draws text at (x, y) with background and foreground colors up to maxWidth.
func drawString(s tcell.Screen, x, y int, text string, bg, fg tcell.Color, maxWidth int) int {
	style := tcell.StyleDefault.Background(bg).Foreground(fg)
	curX := x
	gr := uniseg.NewGraphemes(text)
	for gr.Next() {
		if curX-x >= maxWidth {
			break
		}
		runes := gr.Runes()
		if len(runes) == 0 {
			continue
		}
		mainRune := runes[0]
		comb := runes[1:]
		width := uniseg.StringWidth(gr.Str())
		s.SetContent(curX, y, mainRune, comb, style)
		curX += width
	}
	return curX - x
}

// drawBar fills a horizontal line at y with text and background color.
func drawBar(s tcell.Screen, x, y, width int, text string, bg, fg tcell.Color) {
	style := tcell.StyleDefault.Background(bg).Foreground(fg)
	for col := x; col < x+width; col++ {
		s.SetContent(col, y, ' ', nil, style)
	}
	drawString(s, x, y, text, bg, fg, width)
}

// drawBox draws a single-line or double-line border box from (x1, y1) to (x2, y2).
func drawBox(s tcell.Screen, x1, y1, x2, y2 int, bg, borderCol tcell.Color, title string, titleCol tcell.Color) {
	style := tcell.StyleDefault.Background(bg).Foreground(borderCol)

	// Corners and borders (double line style like Norton Commander)
	topLeft := '╔'
	topRight := '╗'
	bottomLeft := '╚'
	bottomRight := '╝'
	horizontal := '═'
	vertical := '║'

	// Draw interior
	for y := y1; y <= y2; y++ {
		for x := x1; x <= x2; x++ {
			s.SetContent(x, y, ' ', nil, style)
		}
	}

	// Draw top and bottom borders
	for x := x1 + 1; x < x2; x++ {
		s.SetContent(x, y1, horizontal, nil, style)
		s.SetContent(x, y2, horizontal, nil, style)
	}

	// Draw left and right borders
	for y := y1 + 1; y < y2; y++ {
		s.SetContent(x1, y, vertical, nil, style)
		s.SetContent(x2, y, vertical, nil, style)
	}

	// Draw corners
	s.SetContent(x1, y1, topLeft, nil, style)
	s.SetContent(x2, y1, topRight, nil, style)
	s.SetContent(x1, y2, bottomLeft, nil, style)
	s.SetContent(x2, y2, bottomRight, nil, style)

	// Draw title
	if title != "" {
		formattedTitle := " " + title + " "
		titleX := x1 + (x2-x1-len(formattedTitle))/2
		if titleX < x1+2 {
			titleX = x1 + 2
		}
		drawString(s, titleX, y1, formattedTitle, bg, titleCol, x2-x1-4)
	}
}
