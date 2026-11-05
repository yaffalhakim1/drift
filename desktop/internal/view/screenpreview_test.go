package view

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDumpScreenPreview(t *testing.T) {
	if os.Getenv("DRIFT_PREVIEW") == "" {
		t.Skip("set DRIFT_PREVIEW=1 to write previews")
	}

	scroll := &ui.ScrollState{}
	angle := float32(18)

	img := ui.Render(func(c *ui.Context) {
		ui.Themed(c, ThemeFor(c.Theme().Dark)).Children(func() {
			ui.Box(c).Fill().Background(c.Theme().Background).Children(func() {
				ui.Box(c).Width(320).Height(585).Background(c.Theme().Surface).
					Radius(16).Clip().Children(func() {
					ui.Column(c).Fill().Padding(16).Children(func() {
						headerless(c)
						plateSectionStatic(c, angle)
						metaStatic(c)
						seekStatic(c)
						transportStatic(c)
						listStatic(c, scroll)
					})
				})
			})
		})
	}, 320, 585, 2)

	if img == nil {
		t.Fatal("Render returned nil")
	}
	f, err := os.Create("preview-screen.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	t.Log("wrote preview-screen.png")
}

// The preview builds the same sections the real Screen does, with a fixed
// state, so it shows the layout without needing a player or a window.
// headerless draws the real header, with the appearance switch set to dark,
// so the preview shows what the app draws rather than a copy that can drift.
func headerless(c *ui.Context) {
	m := &Model{Theme: "dark"}
	header(c, c.Theme(), m)
}

func plateSectionStatic(c *ui.Context, angle float32) {
	ui.Box(c).Width(148).Height(148).AlignSelf(ui.Center).MarginY(8).Draw(
		func(p *ui.Painter, r ui.Rect) {
			Plate(p, r, PaletteFor(c.Theme()), "Owls of the Night", angle, 0.42)
		})
}

func metaStatic(c *ui.Context) {
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

func seekStatic(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).MarginY(12).Gap(2).Children(func() {
		SeekBar(c, 0.42, func(float64) {})
		ui.Row(c).Children(func() {
			ui.Text(c, "36:09").FontSize(10).TextColor(t.TextMuted).Grow(1)
			ui.Text(c, "−25:03").FontSize(10).TextColor(t.TextMuted)
		})
	})
}

func transportStatic(c *ui.Context) {
	ui.Row(c).AlignItems(ui.Center).Justify(ui.Center).Gap(10).MarginY(10).Children(func() {
		StepButton(c, "prev", "Previous", func() {})
		PlayKey(c, true, func() {})
		StepButton(c, "next", "Next", func() {})
	})
}

func listStatic(c *ui.Context, scroll *ui.ScrollState) {
	t := c.Theme()
	ui.Box(c).Width(288).Height(1).MarginY(8).Background(t.Border)
	l := ui.Scroll(c).FillHeight()
	l.TrackScroll(scroll)

	names := []struct {
		title string
		dur   string
		on    bool
	}{
		{"Owls of the Night", "3:20", true},
		{"Steps", "2:26", false},
		{"amber", "1:26", false},
		{"fever", "2:31", false},
		{"Circle", "1:50", false},
		{"Cuddlin", "2:51", false},
	}
	for i, n := range names {
		el := ui.Row(c).AlignItems(ui.Center).Height(40).Gap(8).PaddingX(6).Radius(8)
		if n.on {
			el.Background(fade(t.Accent, 0.10))
		}
		el.Children(func() {
			if n.on {
				ui.Box(c).Width(22).Draw(func(p *ui.Painter, r ui.Rect) {
					drawBars(p, r, t.Accent, 0.3)
				})
			} else {
				ui.Text(c, pad2(i+18)).FontSize(9.5).TextColor(t.TextMuted)
			}
			col := t.TextMuted
			if n.on {
				col = t.Text
			}
			ui.Text(c, n.title).FontSize(12.5).TextColor(col).MaxLines(1).Grow(1)
			ui.Text(c, n.dur).FontSize(9.5).TextColor(t.TextMuted)
		})
	}
}

func pad2(n int) string {
	s := "0" + itoa(n)
	return s[len(s)-2:]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
