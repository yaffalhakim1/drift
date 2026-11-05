package view

import "github.com/egoist/mygo/ui"

// darkTheme and lightTheme are the built-in themes with Drift's accent.
//
// The accent is Apple system blue, in the variant each appearance needs:
// #0A84FF is Apple's dark-mode blue, and it passes contrast on the dark
// surface. On the light surface Apple's #007AFF falls to 3.79:1, under the
// 4.5:1 AA floor for text, so light uses #0A6FD6 at 4.65:1. Same hue family,
// same feel, readable.
func darkTheme() *ui.Theme {
	t := ui.DarkTheme()
	accent := ui.RGB(0x0a, 0x84, 0xff)
	t.Accent = accent
	t.AccentText = ui.RGB(0xff, 0xff, 0xff)
	return t
}

func lightTheme() *ui.Theme {
	t := ui.LightTheme()
	accent := ui.RGB(0x0a, 0x6f, 0xd6)
	t.Accent = accent
	t.AccentText = ui.RGB(0xff, 0xff, 0xff)
	return t
}

// ThemeFor picks the palette for the system's appearance.
func ThemeFor(dark bool) *ui.Theme {
	if dark {
		return darkTheme()
	}
	return lightTheme()
}
