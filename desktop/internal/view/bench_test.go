package view

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestPlateFrameCost measures many separate frames, each drawing the plate
// once, which is how the app actually runs. A loop inside one Draw callback
// measures nothing, because the painter only records work.
func TestPlateFrameCost(t *testing.T) {
	pal := paletteFor(true)
	var total time.Duration
	var frames int

	var drawAngle float32
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Size(148, 148).Draw(func(p *ui.Painter, r ui.Rect) {
			start := time.Now()
			Plate(p, r, pal, "Owls of the Night", drawAngle, 0.42)
			total += time.Since(start)
			frames++
			drawAngle += 3
		})
	}, 148, 148)

	tester.Frame()
	total, frames = 0, 0

	for i := 0; i < 120; i++ {
		tester.Frame()
	}

	if frames == 0 {
		t.Fatal("no frames drew the plate")
	}
	avg := total / time.Duration(frames)
	t.Logf("%d frames, average plate draw %v per frame", frames, avg)
	if avg > 4*time.Millisecond {
		t.Fatalf("the plate costs %v a frame: it should be cached and rotated", avg)
	}
}
