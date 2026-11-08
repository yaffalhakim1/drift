package view

import (
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"drift/internal/player"
)

// queueScreenTester builds the real Screen in queue mode from a real folder
// when DRIFT_FOLDER points at one, as DRIFT_MIX does for the recording.
func queueScreenTester(t *testing.T) (*ui.Tester, *player.Player, *Model) {
	t.Helper()

	dir := os.Getenv("DRIFT_FOLDER")
	if dir == "" {
		t.Skip("set DRIFT_FOLDER to a folder of mp3s to run the queue screen tests")
	}
	paths, err := player.FolderTracks(dir)
	if err != nil {
		t.Fatalf("FolderTracks: %v", err)
	}

	p := player.New("")
	if err := p.OpenQueue(paths); err != nil {
		t.Fatalf("OpenQueue: %v", err)
	}
	t.Cleanup(p.Close)

	var scroll ui.ScrollState
	var list ui.ListState
	sel := -1 // app-owned choice; the view drives it from the player
	angle := float32(0)
	last := time.Now()
	m := &Model{
		Player: p, Scroll: &scroll, List: &list, ListSelection: &sel,
		Angle: &angle, Last: &last,
		Choose: func() {}, OnPlay: func() {}, OnPrev: func() {},
		OnNext: func() {}, OnSeek: func(float64) {},
	}

	// Themed with Fill, exactly as the app renders (see screen_test.go).
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
	return tester, p, m
}

// TestQueueScreenLaysOut checks the queue screen at real size: the first
// rows exist inside the window, named after the parsed song titles.
func TestQueueScreenLaysOut(t *testing.T) {
	tester, p, _ := queueScreenTester(t)

	tr := p.Tracks()
	if len(tr) == 0 {
		t.Fatal("no tracks")
	}
	first := tr[0].Title
	row, ok := tester.Find(first)
	if !ok {
		t.Fatalf("could not find the first track %q in the queue screen", first)
	}
	if row.Y >= 585 {
		t.Fatalf("first queue row at y=%.0f, below the 585 window", row.Y)
	}
	t.Logf("first of %d tracks at y=%.0f: %q", len(tr), row.Y, first)
}

// TestQueueLifecycle is the thorough regression test for the two queue-list
// bugs. It walks the exact sequence the user hit:
//
//  1. Play from row 0. The list's one choice IS the playing track; no
//     second, stale row may look chosen.
//  2. The first song ends, the player auto-advances to row 1. This is
//     where the list used to lock: the app passes its state by pointer,
//     and a previous version passed a copy, so every frame re-fired the
//     follow and the wheel could never win.
//  3. The user scrolls away mid-song and stays there.
//  4. The song changes again: the list chases.
//
// The construction goes through a pointer into test-owned state, the same
// shape main.go hands the view. A test that built the Model differently
// is what let the copy bug through.
func TestQueueLifecycle(t *testing.T) {
	tester, p, m := queueScreenTester(t)

	tr := p.Tracks()
	if len(tr) < 12 {
		t.Skip("need a dozen tracks to see the window move")
	}

	// --- 1. Track 0 playing: one choice, at the top.
	if got := p.CurrentTrack(); got != 0 {
		t.Fatalf("current track %d, want 0", got)
	}
	if m.ListSelection == nil {
		t.Fatal("ListSelection is nil: the model must hold the app's pointer")
	}
	if *m.ListSelection != 0 {
		t.Fatalf("the list's choice is %d, want the playing track 0", *m.ListSelection)
	}
	f0, _ := m.List.Visible()
	if f0 > 2 {
		t.Fatalf("list starts away from the top: first=%d", f0)
	}

	// --- 2. Auto-advance to track 1, as OnEnd does when a song ends.
	p.JumpTo(1)
	tester.Advance(50 * time.Millisecond)
	tester.Frame()
	tester.Frame()

	if got := *m.ListSelection; got != 1 {
		t.Fatalf("after auto-advance the list's choice is %d, want the playing track 1", got)
	}
	f1, _ := m.List.Visible()
	if f1 > 3 {
		t.Fatalf("after auto-advance the list shows first=%d, want the top rows", f1)
	}

	// --- 3. The user scrolls away mid-song and stays. Frames pass while
	// the music plays; the view must not drag the list back.
	m.List.ScrollIntoView(40)
	tester.Advance(50 * time.Millisecond)
	tester.Frame()
	tester.Frame()

	away, _ := m.List.Visible()
	if away < 30 {
		t.Fatalf("scroll away did not stick: first=%d", away)
	}
	for i := 0; i < 8; i++ {
		tester.Advance(50 * time.Millisecond)
		tester.Frame()
	}
	still, _ := m.List.Visible()
	if still != away {
		t.Fatalf("the list drifted while the song played: %d -> %d", away, still)
	}
	// The choice still names the playing track, not the row scrolled to.
	if got := *m.ListSelection; got != 1 {
		t.Fatalf("scrolling moved the choice to %d, want the playing track 1", got)
	}

	// --- 4. The song changes: the list chases.
	p.JumpTo(len(tr) - 1)
	tester.Advance(50 * time.Millisecond)
	tester.Frame()
	tester.Frame()

	if got := *m.ListSelection; got != len(tr)-1 {
		t.Fatalf("after the jump the list's choice is %d, want %d", got, len(tr)-1)
	}
	f, _ := m.List.Visible()
	if f < len(tr)-4 {
		t.Fatalf("the list did not follow the new song: first=%d, want near %d", f, len(tr)-1)
	}
	t.Logf("lifecycle ok: 0 -> 1 follow, scroll away held at %d over 8 frames, then chased to %d", still, f)
}

// TestQueueScreenScrolls proves the virtual list moves through the queue:
// ScrollIntoView on a row deep in the list must build that row, and
// Visible must report the window moving. The wheel alone is not asserted
// here because glide smoothing spreads a wheel notch over several frames;
// ScrollIntoView is the API the list documents for direct scrolls.
func TestQueueScreenScrolls(t *testing.T) {
	tester, p, m := queueScreenTester(t)

	tr := p.Tracks()
	if len(tr) < 12 {
		t.Skip("need a dozen tracks to see the virtual window move")
	}
	if m.List == nil {
		t.Fatal("model has no ListState")
	}

	first, lastShown := m.List.Visible()
	if lastShown <= first {
		t.Fatalf("the list shows no rows (first=%d last=%d)", first, lastShown)
	}
	t.Logf("visible before: %d..%d of %d", first, lastShown, len(tr))

	// Jump deep into the queue, as a user dragging the scroll bar would.
	// The request lands when the list is next laid out; a settle of frames
	// lets the engine rebuild whatever the view asks for.
	m.List.ScrollIntoView(len(tr) - 1)
	tester.Advance(100 * time.Millisecond)
	tester.Frame()
	tester.Frame()
	tester.Frame()

	f, l := m.List.Visible()
	if f <= first {
		t.Fatalf("ScrollIntoView did not move the window: before %d..%d, after %d..%d",
			first, lastShown, f, l)
	}

	// The row scrolled to must exist and be laid out in the tree. Rows
	// render "Title — Artist" (the same string Label gives assistive
	// technology), so that is what Find must look for.
	name := tr[f].Title
	if tr[f].Artist != "" {
		name = tr[f].Title + " — " + tr[f].Artist
	}
	if _, ok := tester.Find(name); !ok {
		t.Fatalf("row %d (%q) not laid out after ScrollIntoView", f, name)
	}
	t.Logf("visible after: %d..%d, top row %q found", f, l, name)
}
