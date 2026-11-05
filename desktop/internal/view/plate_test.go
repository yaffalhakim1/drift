package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestPlateRenders draws the plate into a headless frame and checks that the
// label and the vinyl put down the pixels they should. A drawing that compiles
// but paints nothing is the failure this catches.
func TestPlateRenders(t *testing.T) {
	const w, h = 148, 148

	img := ui.Render(func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			Plate(p, r, PaletteFor(c.Theme()), "Owls of the Night", 0, 0.42)
		})
	}, w, h, 1)

	if img == nil {
		t.Fatal("Render returned nil")
	}

	b := img.Bounds()
	if b.Dx() != w || b.Dy() != h {
		t.Fatalf("size = %dx%d, want %dx%d", b.Dx(), b.Dy(), w, h)
	}

	// The centre of the disc is the label: light, in both themes.
	lum := luminance(img, w/2, h/2)
	if lum < 0.5 {
		t.Fatalf("centre luminance %.2f: the label should be light", lum)
	}

	// A band well out on the disc is the vinyl. The label ends at dy=36 and
	// the ring is at dy=42, so dy=50 is vinyl. The light theme's vinyl is
	// lighter than the dark theme's, so this asserts the relationship the
	// design depends on rather than one exact value.
	edge := luminance(img, w/2, h/2-50)
	if edge > 0.55 {
		t.Fatalf("edge luminance %.2f: the vinyl should be dark", edge)
	}
	if lum-edge < 0.25 {
		t.Fatalf("label %.2f vs vinyl %.2f: the label must stand out clearly", lum, edge)
	}

	if edge >= lum {
		t.Fatal("the label is not brighter than the vinyl")
	}
}

// TestPlateHandlesEmptyState draws before a recording is chosen.
func TestPlateHandlesEmptyState(t *testing.T) {
	img := ui.Render(func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			Plate(p, r, PaletteFor(c.Theme()), "", 0, 0)
		})
	}, 148, 148, 1)
	if img == nil {
		t.Fatal("Render returned nil with no recording")
	}
}

// TestPlateSurvivesTinyBox guards against a divide-by-zero or a panic when the
// popup is squeezed, which would blank the whole window.
func TestPlateSurvivesTinyBox(t *testing.T) {
	img := ui.Render(func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			Plate(p, r, PaletteFor(c.Theme()), "Nova", 33, 0.1)
		})
	}, 12, 12, 1)
	if img == nil {
		t.Fatal("Render returned nil for a tiny box")
	}
}

// luminance reads a pixel from the rendered frame as 0 to 1.
func luminance(img *image.RGBA, x, y int) float64 {
	r, g, b, a := img.At(x, y).RGBA()
	if a == 0 {
		return 0
	}
	return (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 65535
}
