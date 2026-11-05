// Package mix holds the tracklist: where each chapter of the recording begins,
// and what it is called. It holds no audio. The recording is supplied by the
// user and streamed from wherever they keep it.
package mix

import "time"

// Chapter is one stop in the recording.
type Chapter struct {
	At     time.Duration
	Artist string
	Title  string
}

// Name is what the window and the accessibility label show.
func (c Chapter) Name() string {
	if c.Title == "" {
		return c.Artist
	}
	return c.Title
}

// Label is the full credit, used where there is room.
func (c Chapter) Label() string {
	if c.Title == "" || c.Artist == "" {
		return c.Name()
	}
	return c.Title + " — " + c.Artist
}

// Chapters are the stops in the mix, in order. The offsets are the timestamps
// of the tracklist, verified against the recording: each one sits in the quiet
// gap between two songs.
var Chapters = []Chapter{
	{0, "Intro", ""},
	{15 * time.Second, "WYS", "Snowman"},
	{205 * time.Second, "Fatb", "Cotton Cloud"},
	{328 * time.Second, "rook1e x tender spring", "the places we used to walk"},
	{464 * time.Second, "imagiro", "wool gloves"},
	{628 * time.Second, "Glimlip x Yasper", "I'm sorry"},
	{763 * time.Second, "mell-ø", "Nova"},
	{864 * time.Second, "goosetaf x the fields tape x francis", "carried away"},
	{980 * time.Second, "j'san x epektase", "snow & sand"},
	{1142 * time.Second, "HM Surf", "Single Phial"},
	{1246 * time.Second, "cocabona x Glimlip", "Drops"},
	{1363 * time.Second, "Aso", "espresso"},
	{1510 * time.Second, "Ambulo x mell-ø", "Luminescence"},
	{1610 * time.Second, "DLJ x BIDØ", "Explorers"},
	{1728 * time.Second, "Sarcastic Sounds", "Wish You Were Mine"},
	{1851 * time.Second, "BluntOne", "Reflections"},
	{1968 * time.Second, "Purrple Cat", "Alone Time"},
	{2168 * time.Second, "Kupla", "Owls of the Night"},
	{2308 * time.Second, "dryhope", "Steps"},
	{2454 * time.Second, "ENRA", "amber"},
	{2540 * time.Second, "Psalm Trees", "fever"},
	{2691 * time.Second, "H.1", "Circle"},
	{2801 * time.Second, "Pandrezz", "Cuddlin"},
	{2972 * time.Second, "Jordy Chandra", "Late Night Call"},
	{3104 * time.Second, "less.people", "Gyoza"},
	{3222 * time.Second, "G Mills", "Keyframe"},
	{3392 * time.Second, "mvdb", "breeze"},
	{3486 * time.Second, "Mondo Loops", "Lunar Drive"},
}

// IndexAt returns the chapter that is playing at d.
func IndexAt(d time.Duration) int {
	lo, hi := 0, len(Chapters)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if Chapters[mid].At <= d {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// End is where a chapter stops and the next one starts. The last chapter runs
// to total, which is the length of the recording, not the tracklist: the
// recording is longer than its list of stops.
func End(i int, total time.Duration) time.Duration {
	if i+1 < len(Chapters) {
		return Chapters[i+1].At
	}
	if total > Chapters[i].At {
		return total
	}
	return Chapters[i].At
}

// Length is how long chapter i runs.
func Length(i int, total time.Duration) time.Duration {
	return End(i, total) - Chapters[i].At
}
