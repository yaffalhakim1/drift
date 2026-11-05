package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestThemeGlyphsDiffer checks that the three appearance marks are three
// different drawings.
//
// Two bugs made them identical: the segment ids ("light", "dark") were passed
// straight to a switch that answers to "sun" and "moon", and the sun's rays
// were clipped by the 13 px box, leaving a plain disc like the system mark.
//
// The marks are white on a dark ground, so the comparison reads the red
// channel. Comparing alpha found nothing, because the whole image is opaque.
func TestThemeGlyphsDiffer(t *testing.T) {
	render := func(kind string) []bool {
		img := ui.Render(func(c *ui.Context) {
			c.Root().Background(ui.RGB(0x00, 0x00, 0x00))
			ui.Box(c).Size(13, 13).Draw(func(p *ui.Painter, r ui.Rect) {
				themeGlyph(p, r, kind, ui.RGB(0xff, 0xff, 0xff), ui.RGB(0x00, 0x00, 0x00))
			})
		}, 13, 13, 1)

		marks := make([]bool, 13*13)
		for y := 0; y < 13; y++ {
			for x := 0; x < 13; x++ {
				r, _, _, _ := img.At(x, y).RGBA()
				marks[y*13+x] = r > 0x8000
			}
		}
		return marks
	}

	count := func(marks []bool) int {
		n := 0
		for _, on := range marks {
			if on {
				n++
			}
		}
		return n
	}

	sun := render("sun")
	moon := render("moon")
	auto := render("auto")

	for name, marks := range map[string][]bool{"sun": sun, "moon": moon, "auto": auto} {
		if count(marks) == 0 {
			t.Fatalf("%s drew nothing", name)
		}
	}

	cases := []struct {
		a, b string
		pa   []bool
		pb   []bool
	}{
		{"sun", "moon", sun, moon},
		{"sun", "auto", sun, auto},
		{"moon", "auto", moon, auto},
	}
	for _, c := range cases {
		if equalPixels(c.pa, c.pb) {
			t.Fatalf("%s and %s draw the same marks (%d ink)", c.a, c.b, count(c.pa))
		}
	}
}

func equalPixels(a, b []bool) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
