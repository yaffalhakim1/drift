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
	angle := float32(0)
	last := time.Now()
	m := &Model{
		Player: p, Scroll: &scroll, List: &list, Angle: &angle, Last: &last,
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

// TestQueueScreenFollowsPlaying proves the list follows the song: jump to
// the last track through the player and the list scrolls it into view on
// its own, without a wheel or a programmatic ScrollIntoView from the test.
func TestQueueScreenFollowsPlaying(t *testing.T) {
	tester, p, m := queueScreenTester(t)

	tr := p.Tracks()
	if len(tr) < 12 {
		t.Skip("need a dozen tracks to see the window move")
	}
	if m.List == nil {
		t.Fatal("model has no ListState")
	}

	// A few frames with track 0 playing: the window must sit at the top.
	tester.Frame()
	first, _ := m.List.Visible()
	if first > 2 {
		t.Fatalf("list starts away from the top: first=%d", first)
	}

	// The player jumps to the last song, as if auto-advance ran the queue.
	p.JumpTo(len(tr) - 1)

	// The name the meta line shows must be the new song.
	st := p.State()
	if st.Title != tr[len(tr)-1].Title {
		t.Fatalf("playing %q, want the last track %q", st.Title, tr[len(tr)-1].Title)
	}

	// Build frames: the view must scroll the list to the playing row.
	tester.Advance(100 * time.Millisecond)
	tester.Frame()
	tester.Frame()

	f, l := m.List.Visible()
	if f < len(tr)-4 {
		t.Fatalf("the list did not follow the playing song: visible %d..%d, want near %d",
			f, l, len(tr)-1)
	}
	name := tr[f].Title
	if tr[f].Artist != "" {
		name = tr[f].Title + " — " + tr[f].Artist
	}
	if _, ok := tester.Find(name); !ok {
		t.Fatalf("followed row %d (%q) not laid out", f, name)
	}
	t.Logf("followed to %d..%d, top row %q", f, l, name)
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
