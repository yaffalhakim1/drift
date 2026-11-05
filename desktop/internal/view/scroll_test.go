package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestScrollWithTester lays out the real popup with a real scroll container
// and reads back the list's height after the frame settles. Render only gives
// one pass, which is why measuring through it reported zero.
func TestScrollWithTester(t *testing.T) {
	scroll := &ui.ScrollState{}
	var listRect ui.Rect

	view := func(c *ui.Context) {
		ui.Column(c).Fill().Padding(16).Children(func() {
			ui.Box(c).Height(148)
			ui.Box(c).Height(60)
			ui.Box(c).Height(40)
			ui.Box(c).Height(46)

			el := ui.Scroll(c).Grow(1)
			el.TrackScroll(scroll)
			el.Children(func() {
				for i := 0; i < 28; i++ {
					ui.Box(c).Height(40).Width(288)
				}
			})
			listRect = el.Bounds()
		})
	}

	tester := ui.NewTester(view, 320, 585)
	tester.Frame()
	tester.Frame()

	t.Logf("list at y=%.0f, %.0fx%.0f; content maxY=%.0f",
		listRect.Y, listRect.W, listRect.H, scroll.MaxY)

	if listRect.H <= 0 {
		t.Fatal("the list has no height")
	}
	if listRect.Y+listRect.H > 585 {
		t.Fatalf("the list runs past the window: bottom %.0f", listRect.Y+listRect.H)
	}
	if scroll.MaxY <= listRect.H {
		t.Fatalf("content (%.0f) does not exceed the list (%.0f): nothing to scroll",
			scroll.MaxY, listRect.H)
	}

	// Scroll it the way a wheel would, and confirm the offset moves.
	scroll.Y = 200
	tester.Frame()
	if scroll.Y == 0 {
		t.Fatal("setting the scroll offset was undone: the list is not scrollable")
	}
}
