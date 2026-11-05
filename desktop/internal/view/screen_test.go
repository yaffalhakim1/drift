package view

import (
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"drift/internal/player"
)

// newScreenTester builds the real Screen at the popup's size, with a real
// recording when DRIFT_MIX points at one.
func newScreenTester(t *testing.T) (*ui.Tester, *ui.ScrollState, *Model) {
	t.Helper()

	var p *player.Player
	if path := os.Getenv("DRIFT_MIX"); path != "" {
		if _, err := os.Stat(path); err == nil {
			p = player.New("")
			if err := p.Open(path); err != nil {
				t.Fatalf("Open: %v", err)
			}
			t.Cleanup(p.Close)
		}
	}
	if p == nil {
		t.Skip("set DRIFT_MIX to a recording to run the screen tests")
	}

	var scroll ui.ScrollState
	angle := float32(0)
	last := time.Now()
	m := &Model{
		Player: p, Scroll: &scroll, Angle: &angle, Last: &last,
		Choose: func() {}, OnPlay: func() {}, OnPrev: func() {},
		OnNext: func() {}, OnSeek: func(float64) {},
	}

	// Themed with Fill, exactly as the app renders: without Fill the column
	// takes its content's height and the list cannot scroll.
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Themed(c, ThemeFor(c.Theme().Dark)).Fill().Children(func() {
			c.Root().Background(c.Theme().Background)
			ui.Column(c).Fill().Padding(16).Children(func() {
				Screen(c, m)
			})
		})
	}, 320, 585)

	tester.Frame()
	tester.Frame()
	return tester, &scroll, m
}

// TestScreenScrolls is the end-to-end scroll check: the real screen, the real
// player, a real wheel event. Two things had to be true for this to pass,
// and both were false in the app:
//
//  1. No ui.Themed wrapper around the screen. A themed subtree sizes to its
//     content, which removed the height the list needed to scroll against,
//     and the popup grew to 1557 px inside a 585 px window.
//  2. The wheel reports dy positive downwards; a negative dy does nothing.
func TestScreenScrolls(t *testing.T) {
	tester, scroll, _ := newScreenTester(t)

	if scroll.MaxY <= 0 {
		t.Fatalf("the chapter list has no scrollable content (maxY=%.0f)", scroll.MaxY)
	}

	// The first chapter sits at the top of the list; wheel just below it.
	row, ok := tester.Find("Snowman — WYS")
	if !ok {
		t.Fatal("could not find a chapter row to aim at")
	}
	y := row.Y + 60

	before := scroll.Y
	tester.Wheel(150, y, 0, 60)
	tester.Advance(300 * time.Millisecond)
	tester.Frame()

	if scroll.Y <= before {
		t.Fatalf("the wheel did not scroll: y stayed %.0f (content = %.0f)", scroll.Y, scroll.MaxY)
	}
	t.Logf("scrolled to %.0f of %.0f", scroll.Y, scroll.MaxY)

	// And back up.
	up := scroll.Y
	tester.Wheel(150, y, 0, -60)
	tester.Advance(300 * time.Millisecond)
	tester.Frame()
	if scroll.Y >= up {
		t.Fatalf("the wheel did not scroll back up: y stayed %.0f", scroll.Y)
	}
}

// TestScreenFitsWindow guards the bug that came before the scroll fix: the
// popup must not lay itself out taller than the window it lives in.
func TestScreenFitsWindow(t *testing.T) {
	tester, _, _ := newScreenTester(t)

	row, ok := tester.Find("Snowman — WYS")
	if !ok {
		t.Skip("no recording to lay out")
	}
	if row.Y >= 585 {
		t.Fatalf("the first chapter row is at y=%.0f, below the 585 window", row.Y)
	}
	if row.Y < 300 {
		t.Fatalf("the first chapter row is at y=%.0f, overlapping the controls above it", row.Y)
	}
	t.Logf("first chapter at y=%.0f", row.Y)
}
