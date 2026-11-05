package view

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

// Clock formats a duration as m:ss, or h:mm:ss past an hour.
func Clock(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	s := int(d.Round(time.Second) / time.Second)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, (s%3600)/60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// progressOf maps a position onto 0 to 1.
func progressOf(pos, total time.Duration) float64 {
	if total <= 0 {
		return 0
	}
	p := float64(pos) / float64(total)
	switch {
	case p < 0:
		return 0
	case p > 1:
		return 1
	}
	return p
}

// PlayKey is the one bright control: a ButtonBase, so it takes the focus,
// answers Enter and Space, and reads as a button to assistive technology,
// wearing the accent gradient only this control gets.
func PlayKey(c *ui.Context, playing bool, onPress func()) {
	t := c.Theme()

	top, bottom, glyph := ui.RGB(0xf8, 0xf3, 0xe8), ui.RGB(0xe6, 0xdd, 0xca), ui.RGB(0x14, 0x14, 0x0f)
	if !t.Dark {
		top, bottom, glyph = ui.RGB(0x2a, 0x26, 0x20), ui.RGB(0x12, 0x10, 0x0d), ui.RGB(0xf7, 0xf2, 0xe6)
	}

	btn := ui.ButtonBase(c.Key("play")).Size(46, 46).Radius(23).Center()
	// The press animation is a scale about the button's own centre, which is
	// what the base's focus ring and hit box stay aligned with.
	scale := btn.Spring("press", b2f(!btn.Pressed()), ui.Snappy.Spring)
	btn.Scale(scale, scale)
	btn.Gradient(top, bottom, 180)
	btn.Shadow(0, 3, 10, 0, ui.RGBA(0, 0, 0, 0.32))

	btn.Children(func() {
		if playing {
			// Two bars, drawn rather than an icon file: the pause mark is two
			// rectangles and no icon set should be shipped for it.
			ui.Row(c).Gap(3).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				ui.Box(c).Size(3.2, 12).Radius(1.6).Background(glyph)
				ui.Box(c).Size(3.2, 12).Radius(1.6).Background(glyph)
			})
		} else {
			playTriangle(c, glyph)
		}
	})

	if playing {
		btn.Label("Pause")
	} else {
		btn.Label("Play")
	}
	if btn.Clicked() {
		onPress()
	}
}

// playTriangle draws the play mark: a triangle pointing right, with the
// optical nudge that keeps it centred in a circle.
func playTriangle(c *ui.Context, col ui.Color) {
	ui.Box(c).Width(11).Height(13).Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		glyphPath.Reset().
			MoveTo(cx-2, cy-6.5).
			LineTo(cx-2, cy+6.5).
			LineTo(cx+6, cy).
			Close()
		p.FillPath(glyphPath, col)
	})
}

// StepButton is a small transport button built on ButtonBase.
func StepButton(c *ui.Context, icon, label string, onPress func()) {
	t := c.Theme()

	btn := ui.ButtonBase(c.Key(label)).Size(34, 34).Radius(17).Center()
	col := t.TextMuted
	if btn.Hovered() || btn.Pressed() {
		col = t.Text
	}

	btn.Children(func() {
		ui.Box(c).Size(16, 16).Draw(func(p *ui.Painter, r ui.Rect) {
			stepGlyph(p, r, icon, col)
		})
	})

	btn.Label(label)
	if btn.Clicked() {
		onPress()
	}
}

// stepGlyph draws the previous or next mark: a triangle and a bar, at the
// proportions of the web build's icons.
func stepGlyph(p *ui.Painter, r ui.Rect, icon string, col ui.Color) {
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	triW, halfH := float32(4.8), float32(4.4)

	if icon == "prev" {
		glyphPath.Reset().
			MoveTo(cx+triW, cy-halfH).
			LineTo(cx+triW, cy+halfH).
			LineTo(cx-1, cy).
			Close()
		p.FillPath(glyphPath, col)
		p.Line(cx-triW, cy-halfH, cx-triW, cy+halfH, 1.3, col)
		return
	}
	glyphPath.Reset().
		MoveTo(cx-triW, cy-halfH).
		LineTo(cx-triW, cy+halfH).
		LineTo(cx+1, cy).
		Close()
	p.FillPath(glyphPath, col)
	p.Line(cx+triW, cy-halfH, cx+triW, cy+halfH, 1.3, col)
}

// SeekBar is a SliderBase, so dragging, clicking the track, the arrows, Page
// Up and Down, Home and End and assistive technology all come with it. Only
// the look is ours.
func SeekBar(c *ui.Context, progress float64, onSeek func(float64)) {
	t := c.Theme()

	// Bound to a value the caller owns each frame, so the base has something
	// to set; the position itself comes from the player.
	var value float64 = progress * 1000
	el := ui.SliderBase(c.Key("seek"), &value, 0, 1000).Height(20).PaddingX(6)

	if el.Changed() {
		onSeek(value / 1000)
	}

	el.Draw(func(p *ui.Painter, r ui.Rect) {
		y := r.Y + r.H/2
		const thumbR = float32(6)
		trackX := r.X + 6
		trackW := r.W - 12

		p.Fill(ui.Rect{X: trackX, Y: y - 1.5, W: trackW, H: 3}, t.Border, 1.5)

		at := float32(value / 1000)
		if at > 0 {
			p.Fill(ui.Rect{X: trackX, Y: y - 1.5, W: trackW * at, H: 3}, t.Accent, 1.5)
		}

		kx := trackX + trackW*at
		p.Fill(ui.Rect{X: kx - thumbR, Y: y - thumbR, W: thumbR * 2, H: thumbR * 2},
			ui.RGB(0xf6, 0xf1, 0xe6), thumbR)
	})

	el.Label("Position")
}

// b2f turns a bool into the spring target the press animation expects.
func b2f(on bool) float32 {
	if on {
		return 1
	}
	return 0
}

// glyphPath is reused between frames: building a path each time would
// allocate for nothing, and these glyphs redraw on every frame.
var glyphPath = &ui.Path{}
