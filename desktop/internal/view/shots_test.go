package view

import (
	"fmt"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"drift/internal/mix"
)

// TestDumpScreenshots writes the screenshots the README uses: the popup in
// both appearances, with real chapter data, at three times the display scale.
// The landing page shows the popup at up to 480 CSS px, so 3x keeps it sharp
// on a 1x screen and on a 2x one.
func TestDumpScreenshots(t *testing.T) {
	if os.Getenv("DRIFT_SHOTS") == "" {
		t.Skip("set DRIFT_SHOTS=1 to write screenshots")
	}

	type shot struct {
		name  string
		dark  bool
		theme string
	}
	shots := []shot{
		{"popup-dark", true, "dark"},
		{"popup-light", false, "light"},
	}

	for _, s := range shots {
		img := ui.Render(func(c *ui.Context) {
			ui.Themed(c, ThemeFor(s.dark)).Fill().Children(func() {
				c.Root().Background(c.Theme().Background)
				ui.Box(c).Width(320).Height(585).
					Background(c.Theme().Surface).Radius(16).Clip().Children(func() {
					ui.Column(c).Fill().Padding(16).Children(func() {
						previewHeader(c, s.dark)
						previewPlate(c, s.dark)
						previewMeta(c)
						previewSeek(c)
						previewTransport(c)
						previewList(c)
					})
				})
			})
		}, 320, 585, 3)

		f, err := os.Create(s.name + ".png")
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote %s.png", s.name)
	}
}

// previewHeader draws the header with one appearance chosen, so the two
// screenshots show different segments lit.
func previewHeader(c *ui.Context, dark bool) {
	chosen := 2
	if !dark {
		chosen = 1
	}
	ui.Row(c).AlignItems(ui.Center).Height(30).Children(func() {
		ui.Text(c, "drift").FontSize(15).FontWeight(600).LetterSpacing(-0.2).Grow(1)

		seg := ui.SegmentedBase(c, &chosen, 3)
		seg.Track.Gap(1).Padding(2).Radius(14).Background(c.Theme().SurfaceHover).Children(func() {
			for i, glyph := range []string{"auto", "sun", "moon"} {
				s := seg.Segment(i).Size(24, 24).Center().Radius(12)
				if i == chosen {
					s.Background(c.Theme().Surface)
				}
				col := c.Theme().TextMuted
				if i == chosen {
					col = c.Theme().Accent
				}
				s.Children(func() {
					ui.Box(c).Size(13, 13).Draw(func(p *ui.Painter, r ui.Rect) {
						themeGlyph(p, r, glyph, col, c.Theme().Surface)
					})
				})
			}
		})
	})
}

func previewPlate(c *ui.Context, dark bool) {
	ui.Box(c).Width(148).Height(148).AlignSelf(ui.Center).MarginY(10).Draw(
		func(p *ui.Painter, r ui.Rect) {
			Plate(p, r, paletteFor(dark), "Owls of the Night", 18, 0.61)
		})
}

func previewMeta(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Gap(2).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(5, 5).Radius(3).Background(t.Accent)
			ui.Text(c, "NOW PLAYING").FontSize(9.5).LetterSpacing(1.4).TextColor(t.TextMuted)
		})
		ui.Text(c, "Owls of the Night").FontSize(20).FontWeight(600).TextColor(t.Text)
		ui.Text(c, "Kupla").FontSize(11.5).TextColor(t.TextMuted)
	})
}

func previewSeek(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).MarginY(12).Gap(2).Children(func() {
		SeekBar(c, 0.61, func(float64) {})
		ui.Row(c).Children(func() {
			ui.Text(c, "36:09").FontSize(10).TextColor(t.TextMuted).Grow(1)
			ui.Text(c, "−25:03").FontSize(10).TextColor(t.TextMuted)
		})
	})
}

func previewTransport(c *ui.Context) {
	ui.Row(c).AlignItems(ui.Center).Justify(ui.Center).Gap(10).MarginY(10).Children(func() {
		StepButton(c, "prev", "Previous", func() {})
		PlayKey(c, true, func() {})
		StepButton(c, "next", "Next", func() {})
	})
}

func previewList(c *ui.Context) {
	t := c.Theme()
	ui.Box(c).Width(288).Height(1).MarginY(8).Background(t.Border)

	// The rows start at chapter 18 and the lengths come from mix.Length, the
	// same call the screen makes. Hardcoded lengths drifted from the data:
	// they said 3:20 for a chapter that runs 2:20, and a screenshot of other
	// people's work should not carry numbers that are wrong.
	const first, last = 17, 22

	// Total is the length the popup shows on a 61 minute recording, so the
	// last visible row has a length to report.
	total := 61*time.Minute + 12*time.Second

	var scroll ui.ScrollState
	list := ui.Scroll(c).Grow(1)
	list.TrackScroll(&scroll)
	list.Children(func() {
		for i := first; i <= last; i++ {
			on := i == 17
			el := ui.Row(c).AlignItems(ui.Center).Height(40).Gap(8).PaddingX(6).Radius(8)
			if on {
				el.Background(fade(t.Accent, 0.10))
			}
			el.Children(func() {
				if on {
					ui.Box(c).Width(22).Draw(func(p *ui.Painter, rr ui.Rect) {
						drawBars(p, rr, t.Accent, 0.32)
					})
				} else {
					ui.Text(c, fmt.Sprintf("%02d", i+1)).FontSize(9.5).TextColor(t.TextMuted)
				}
				col := t.TextMuted
				if on {
					col = t.Text
				}
				ui.Text(c, mix.Chapters[i].Name()).FontSize(12.5).TextColor(col).MaxLines(1).Grow(1)
				ui.Text(c, Clock(mix.Length(i, total))).FontSize(9.5).TextColor(t.TextMuted)
			})
		}
	})
}
