package view

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestLoopKeepsFramesComing checks that Loop is the API that drives continuous
// animation: it must return a moving value and leave the window asking for
// another frame. The plate and the bars both depend on this.
func TestLoopKeepsFramesComing(t *testing.T) {
	var phases []float32

	tester := ui.NewTester(func(c *ui.Context) {
		el := ui.Box(c).Size(40, 40)
		phase := el.Loop("spin", time.Second, ui.Linear)
		phases = append(phases, phase)
		el.Draw(func(p *ui.Painter, r ui.Rect) {
			drawBars(p, r, c.Theme().Accent, phase)
		})
	}, 40, 40)

	tester.Frame()
	if !tester.FramePending() {
		t.Fatal("after one frame the window did not ask for another: Loop is not animating")
	}

	first := phases[len(phases)-1]
	tester.Advance(250 * time.Millisecond)
	tester.Frame()
	later := phases[len(phases)-1]

	if later <= first {
		t.Fatalf("Loop progress did not advance: %.3f then %.3f", first, later)
	}
	t.Logf("Loop advanced %.3f -> %.3f over 250ms, and kept asking for frames", first, later)

	// A full period returns to the start rather than running past it.
	tester.Advance(1200 * time.Millisecond)
	tester.Frame()
	if p := phases[len(phases)-1]; p < 0 || p >= 1 {
		t.Fatalf("Loop progress is %.3f, outside 0..1", p)
	}
}
