package ui

import "github.com/gdamore/tcell/v2"

// Theme defines the color scheme for the application.
type Theme struct {
	PanelBg          tcell.Color
	PanelFg          tcell.Color
	PanelBorder      tcell.Color
	PanelTitle       tcell.Color
	HeaderBg         tcell.Color
	HeaderFg         tcell.Color
	CursorBg         tcell.Color
	CursorFg         tcell.Color
	SelectedFg       tcell.Color
	SelectedBg       tcell.Color
	DirFg            tcell.Color
	ArchiveFg        tcell.Color
	ExecutableFg     tcell.Color
	StatusBarBg      tcell.Color
	StatusBarFg      tcell.Color
	KeyBarNumberBg   tcell.Color
	KeyBarNumberFg   tcell.Color
	KeyBarTextBg     tcell.Color
	KeyBarTextFg     tcell.Color
	DialogBg         tcell.Color
	DialogFg         tcell.Color
	DialogBorder     tcell.Color
	CommandPromptFg  tcell.Color
	CommandInputFg   tcell.Color
	MenuBarBg        tcell.Color
	MenuBarFg        tcell.Color
	MenuBarActiveBg  tcell.Color
	MenuBarActiveFg  tcell.Color
}

// ClassicBlueTheme returns the traditional Norton/Total Commander blue theme.
func ClassicBlueTheme() *Theme {
	return &Theme{
		PanelBg:          tcell.ColorNavy,
		PanelFg:          tcell.ColorWhite,
		PanelBorder:      tcell.ColorAqua,
		PanelTitle:       tcell.ColorYellow,
		HeaderBg:         tcell.ColorDarkBlue,
		HeaderFg:         tcell.ColorYellow,
		CursorBg:         tcell.ColorTeal,
		CursorFg:         tcell.ColorBlack,
		SelectedFg:       tcell.ColorYellow,
		SelectedBg:       tcell.ColorNavy,
		DirFg:            tcell.ColorAqua,
		ArchiveFg:        tcell.ColorFuchsia,
		ExecutableFg:     tcell.ColorLime,
		StatusBarBg:      tcell.ColorTeal,
		StatusBarFg:      tcell.ColorBlack,
		KeyBarNumberBg:   tcell.ColorBlack,
		KeyBarNumberFg:   tcell.ColorYellow,
		KeyBarTextBg:     tcell.ColorTeal,
		KeyBarTextFg:     tcell.ColorBlack,
		DialogBg:         tcell.ColorGray,
		DialogFg:         tcell.ColorBlack,
		DialogBorder:     tcell.ColorWhite,
		CommandPromptFg:  tcell.ColorYellow,
		CommandInputFg:   tcell.ColorWhite,
		MenuBarBg:        tcell.ColorTeal,
		MenuBarFg:        tcell.ColorBlack,
		MenuBarActiveBg:  tcell.ColorNavy,
		MenuBarActiveFg:  tcell.ColorYellow,
	}
}
