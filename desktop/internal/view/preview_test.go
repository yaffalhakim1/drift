package view

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestDumpPlatePreview writes what the plate looks like, for a human to check.
// Skipped unless DRIFT_PREVIEW is set, so it never slows the normal suite.
func TestDumpPlatePreview(t *testing.T) {
	if os.Getenv("DRIFT_PREVIEW") == "" {
		t.Skip("set DRIFT_PREVIEW=1 to write previews")
	}

	for _, dark := range []bool{true, false} {
		img := ui.Render(func(c *ui.Context) {
			// The plate's material is chosen from the theme, so force the
			// appearance for the light pass or both previews look identical.
			pal := paletteFor(true)
			if !dark {
				pal = paletteFor(false)
			}
			ui.Box(c).Fill().Background(c.Theme().Surface).Draw(func(p *ui.Painter, r ui.Rect) {
				Plate(p, r, pal, "Owls of the Night", 18, 0.42)
			})
		}, 200, 200, 2)

		name := "preview-plate-light.png"
		if dark {
			name = "preview-plate-dark.png"
		}
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote %s", name)
	}
}
