package view

import (
	"github.com/egoist/mygo/ui"
)

// Palette is the handful of colours the plate needs beyond the theme.
type Palette struct {
	DiscIn, DiscOut ui.Color
	Groove          ui.Color
	Sheen           ui.Color
	LabelTop        ui.Color
	LabelLow        ui.Color
	LabelRing       ui.Color
	LabelInk        ui.Color
	Accent          ui.Color
	Shadow          ui.Color
}

// PaletteFor picks the plate's material from the app theme. The disc stays
// dark in both: vinyl does not turn cream in daylight.
func PaletteFor(t *ui.Theme) Palette { return paletteFor(t.Dark) }

func paletteFor(dark bool) Palette {
	if dark {
		return Palette{
			DiscIn:    ui.RGB(0x1a, 0x1a, 0x1f),
			DiscOut:   ui.RGB(0x0a, 0x0a, 0x0d),
			Groove:    ui.RGBA(0xff, 0xff, 0xff, 0.10),
			Sheen:     ui.RGBA(0xff, 0xff, 0xff, 0.10),
			LabelTop:  ui.RGB(0xf4, 0xec, 0xdc),
			LabelLow:  ui.RGB(0xde, 0xd0, 0xb8),
			LabelRing: ui.RGB(0xc8, 0xb8, 0x9b),
			LabelInk:  ui.RGB(0x2b, 0x25, 0x1f),
			Accent:    ui.RGB(0x0a, 0x84, 0xff),
			Shadow:    ui.RGBA(0x00, 0x00, 0x00, 0.44),
		}
	}
	return Palette{
		DiscIn:    ui.RGB(0x2c, 0x2c, 0x33),
		DiscOut:   ui.RGB(0x13, 0x13, 0x16),
		Groove:    ui.RGBA(0xff, 0xff, 0xff, 0.14),
		Sheen:     ui.RGBA(0xff, 0xff, 0xff, 0.14),
		LabelTop:  ui.RGB(0xfd, 0xfa, 0xf3),
		LabelLow:  ui.RGB(0xf0, 0xe5, 0xd1),
		LabelRing: ui.RGB(0xd7, 0xc8, 0xac),
		LabelInk:  ui.RGB(0x2f, 0x28, 0x20),
		Accent:    ui.RGB(0x0a, 0x6f, 0xd6),
		Shadow:    ui.RGBA(0x00, 0x00, 0x00, 0.24),
	}
}

// Plate draws the record into the element's box.
//
// The disc turns and its print does not, so the title stays readable. The
// progress ring sits on the frame rather than the disc, so its twelve o'clock
// start does not drift as the disc spins.
func Plate(p *ui.Painter, b ui.Rect, pal Palette, title string, angle float32, progress float64) {
	side := b.W
	if b.H < side {
		side = b.H
	}
	if side <= 8 {
		return
	}

	cx, cy := b.X+b.W/2, b.Y+b.H/2
	rOut := side / 2
	rLabel := rOut * 0.44
	rRing := rLabel + rOut*0.065

	disc := ui.Rect{X: cx - rOut, Y: cy - rOut, W: side, H: side}

	// A soft shadow under the disc, so it reads as an object on the card.
	p.Shadow(disc, rOut, 0, 5, 12, 0, pal.Shadow)

	// Disc: darker at the rim, so the grooves have something to sit on.
	p.FillRadialGradient(disc, ui.RadialGradient{
		Center: ui.Point{X: 0.42, Y: 0.38},
		Radius: rOut * 1.1,
		Stops: []ui.GradientStop{
			{At: 0, Color: pal.DiscIn},
			{At: 0.58, Color: pal.DiscIn},
			{At: 1, Color: pal.DiscOut},
		},
	}, rOut)

	// Grooves: faint bands, wider apart toward the rim.
	const grooves = 18
	for i := 0; i < grooves; i++ {
		t := float32(i) / float32(grooves-1)
		r := rLabel + rOut*0.12 + (rOut-rLabel-rOut*0.16)*t
		width := float32(0.7)
		if i%4 == 0 {
			width = 1
		}
		p.Stroke(ui.Rect{X: cx - r, Y: cy - r, W: r * 2, H: r * 2}, pal.Groove, r, width)
	}

	// One sheen across the disc, clipped to it so it cannot spill onto the card.
	p.Clip(disc, rOut, func() {
		p.FillGradient(disc, ui.LinearGradient{
			From:  pal.Sheen,
			To:    ui.RGBA(0xff, 0xff, 0xff, 0),
			Angle: 215,
			Start: 0,
			End:   0.62,
		}, rOut)
	})

	// The label turns with the disc, around the disc's centre.
	p.Transform(ui.Rotation(angle), cx, cy, func() {
		label := ui.Rect{X: cx - rLabel, Y: cy - rLabel, W: rLabel * 2, H: rLabel * 2}
		p.FillRadialGradient(label, ui.RadialGradient{
			Center: ui.Point{X: 0.34, Y: 0.3},
			Radius: rLabel * 1.5,
			Stops: []ui.GradientStop{
				{At: 0, Color: pal.LabelTop},
				{At: 1, Color: pal.LabelLow},
			},
		}, rLabel)
		p.Stroke(label, pal.LabelRing, rLabel, 1)
	})

	// Track ring, then the progress arc over it, clockwise from twelve.
	p.Stroke(ui.Rect{X: cx - rRing, Y: cy - rRing, W: rRing * 2, H: rRing * 2},
		fade(pal.LabelRing, 0.35), rRing, 1.2)
	if progress > 0.001 {
		arcPath.Reset().Circle(cx, cy, rRing).Trim(0, float32(progress))
		p.StrokePath(arcPath, 1.8, pal.Accent)
	}

	// The print: the title, shrunk and then shortened to stay on the label.
	// It does not turn, so it can be read at any moment.
	if title == "" {
		return
	}
	size := rLabel * 0.19
	if size > 11 {
		size = 11
	}
	if size < 8 {
		size = 8
	}
	// The print may use most of the label, but must stay clear of the ring.
	// 1.55 radii leaves about 8 px of air at 148 px, which reads as a margin
	// rather than as text running into the border.
	maxW := rLabel * 1.55
	w := textWidth(p, title, size)
	for size > 6 && w > maxW {
		size -= 0.25
		w = textWidth(p, title, size)
	}
	if w > maxW {
		title = shorten(p, title, size, maxW)
		w = textWidth(p, title, size)
	}
	p.Text(cx-w/2, cy+size*0.36, title, size, pal.LabelInk)
}

// arcPath is reused between frames: building a ring every frame would
// allocate for no reason, and the plate redraws while the music plays.
var arcPath = &ui.Path{}

// textWidth measures one run of text at a size, with the same painter that
// draws it, so measurement and drawing cannot disagree.
func textWidth(p *ui.Painter, s string, size float32) float32 {
	w, _ := p.MeasureText(1000, ui.Span{Text: s, Size: size})
	return w
}

// shorten trims s with an ellipsis until it fits maxW at size.
func shorten(p *ui.Painter, s string, size, maxW float32) string {
	r := []rune(s)
	for len(r) > 1 && textWidth(p, string(r)+"…", size) > maxW {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// fade scales a colour's alpha by f.
func fade(c ui.Color, f float32) ui.Color {
	return ui.RGBA(c.R, c.G, c.B, float32(c.A)/255*f)
}
