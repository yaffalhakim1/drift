package view

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"drift/internal/mix"
	"drift/internal/player"
)

// Model is everything the screen needs: the player, the scroll position of the
// chapter list, and the callbacks the controls fire.
type Model struct {
	Player *player.Player

	// Scroll keeps the chapter list where the user left it between frames.
	Scroll *ui.ScrollState

	// List keeps the queue list's place. Its fields (Key, Label, Selected)
	// are set before building, per docs/ui/views.md: state lives in the
	// app, never in the view.
	List *ui.ListState

	// Angle turns the plate, in degrees.
	Angle *float32

	// Last is the frame time the angle was advanced to, so rotation follows
	// the clock rather than a fixed step and survives a dropped frame.
	Last *time.Time

	// Choose asks the user for a recording, from the empty state.
	Choose func()

	// The queue list's chosen row. Lives in the model, pointed at by the
	// ListState; -1 is none.
	listSelection int

	// following is the track index the list last scrolled to, so the view
	// scrolls only when the playing song changes, not on every frame.
	following int

	// Theme is the appearance the user picked: "system", "light" or "dark".
	Theme string

	OnTheme func(next string)

	OnPlay func()
	OnPrev func()
	OnNext func()
	OnSeek func(fraction float64)
}

// Screen builds the popup at the size the flyout gives it.
//
// The column is a grid of fixed rows plus one flexible row: the chapter list
// takes what is left. Without a bounded height the list cannot scroll, which
// is why the tracklist appeared frozen at the first few chapters.
func Screen(c *ui.Context, m *Model) {
	t := c.Theme()
	st := m.Player.State()

	if !st.Loaded {
		ui.Column(c).Fill().Padding(16).Children(func() {
			emptyState(c, t, m)
		})
		return
	}

	ui.Column(c).Fill().Padding(16).Gap(0).Children(func() {
		header(c, t, m)
		plateSection(c, m, st)
		trackMeta(c, t, st)
		position(c, t, m, st)
		transport(c, m, st)
		chapterList(c, t, m, st)
	})
}

// blend mixes two colours, t of the way from a to b.
func blend(a, b ui.Color, t float32) ui.Color {
	mix := func(x, y uint8) uint8 {
		return uint8(float32(x)*(1-t) + float32(y)*t)
	}
	return ui.RGBA(mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), 1)
}

// header is the wordmark and the appearance switch.
//
// The switch is a SegmentedBase: a radio group to assistive technology, with
// the arrows moving the choice and the focus, which is what a three-way
// control should be. Only its look is ours.
func header(c *ui.Context, t *ui.Theme, m *Model) {
	ui.Row(c).AlignItems(ui.Center).Height(30).Children(func() {
		ui.Text(c, "drift").FontSize(15).FontWeight(600).LetterSpacing(-0.2).Grow(1)

		names := []string{"Follow the system appearance", "Light appearance", "Dark appearance"}
		ids := []string{"system", "light", "dark"}

		chosen := 0
		for i, id := range ids {
			if m.Theme == id {
				chosen = i
			}
		}

		seg := ui.SegmentedBase(c.Key("theme"), &chosen, 3)
		seg.Track.Gap(1).Padding(2).Radius(14).Background(t.SurfaceHover).Children(func() {
			for i := range ids {
				s := seg.Segment(i).Size(24, 24).Center().Radius(12)
				if i == chosen {
					s.Background(t.Surface)
					s.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.18))
				}
				col := t.TextMuted
				if i == chosen {
					col = t.Accent
				}
				s.Children(func() {
					ui.Box(c).Size(13, 13).Draw(func(p *ui.Painter, r ui.Rect) {
						// The ids are "system", "light" and "dark"; the glyphs
						// answer to "auto", "sun" and "moon". Mapping them here
						// is what keeps the three marks distinct: passing the id
						// straight through made every icon the default.
						glyph := map[string]string{
							"system": "auto",
							"light":  "sun",
							"dark":   "moon",
						}[ids[i]]
						themeGlyph(p, r, glyph, col, t.Surface)
					})
				})
				s.Label(names[i])
			}
		})
		seg.Track.Label("Appearance")

		// Changed reports a new choice, so the model hears once per change.
		if seg.Track.Changed() && m.OnTheme != nil && chosen >= 0 && chosen < len(ids) {
			m.OnTheme(ids[chosen])
		}
	})
}

// themeGlyph draws the three appearance marks, sized to a 13 px box.
//
// Each mark must stay inside its box: the sun's rays at 5.8 px from the centre
// were clipped by a 13 px box and the mark read as a plain disc, the same as
// the system mark. Everything here is drawn within 5.5 px of the centre.
func themeGlyph(p *ui.Painter, r ui.Rect, kind string, col, bg ui.Color) {
	cx, cy := r.X+r.W/2, r.Y+r.H/2

	switch kind {
	case "sun":
		// A disc with eight rays, the rays short enough to fit.
		p.Fill(ui.Rect{X: cx - 2.2, Y: cy - 2.2, W: 4.4, H: 4.4}, col, 2.2)
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			dx, dy := float32(math.Cos(a)), float32(math.Sin(a))
			p.Line(cx+dx*3.6, cy+dy*3.6, cx+dx*5.2, cy+dy*5.2, 1.1, col)
		}
	case "moon":
		// A crescent: a disc with a second disc bitten out of its upper right.
		p.Fill(ui.Rect{X: cx - 4.4, Y: cy - 4.4, W: 8.8, H: 8.8}, col, 4.4)
		p.Fill(ui.Rect{X: cx - 1.6, Y: cy - 5.4, W: 8.8, H: 8.8}, bg, 4.4)
	default:
		// System: a ring with its right half filled, the half-dark disc that
		// reads as "follow the system" everywhere it is used.
		p.Stroke(ui.Rect{X: cx - 4.4, Y: cy - 4.4, W: 8.8, H: 8.8}, col, 4.4, 1.1)
		p.Fill(ui.Rect{X: cx, Y: cy - 4.4, W: 4.4, H: 8.8}, col, 0)
	}
}

// plateSection draws the record and advances its angle while playing.
//
// Rotation is advanced inside Draw and repainted with Painter.AnimationFrame,
// not Context.AnimationFrame. The difference is the whole cost of the
// animation: the context version rebuilds every element of the popup for
// every frame, which is what made the disc and the bars stutter. The painter
// version paints this one element again and leaves the layout alone.
// plateSection draws the record and turns it while the music plays.
//
// The rotation comes from Loop, not from a painter repaint request. Loop is
// the API meant for this: it returns the progress of an animation that keeps
// restarting, and it keeps frames coming while the element is built, so the
// element is redrawn on the normal frame path instead of through a repaint
// that the engine may reject. One turn takes 12 s, so the disc reads as
// turning without becoming a fan.
func plateSection(c *ui.Context, m *Model, st player.State) {
	el := ui.Box(c).Width(148).Height(148).AlignSelf(ui.Center).MarginY(10)

	title := queueTitle(st)
	if !st.Queue && st.Chapter >= 0 && st.Chapter < len(mix.Chapters) {
		title = mix.Chapters[st.Chapter].Name()
	}
	pal := PaletteFor(c.Theme())

	var angle float32
	if st.Playing && !c.Preferences().ReduceMotion {
		angle = el.Loop("turn", 12*time.Second, ui.Linear) * 360
	}

	el.Draw(func(p *ui.Painter, r ui.Rect) {
		Plate(p, r, pal, title, angle, progressOf(st.Position, st.Total))
	})
}

// queueTitle is what the plate and the meta line show in queue mode: the
// song's title, or a placeholder while nothing plays.
func queueTitle(st player.State) string {
	if st.Title != "" {
		return st.Title
	}
	return "Nothing playing"
}

// trackMeta is the chapter title and its artist.
func trackMeta(c *ui.Context, t *ui.Theme, st player.State) {
	title, artist := queueTitle(st), st.Artist
	if !st.Queue {
		ch := mix.Chapters[st.Chapter]
		title, artist = ch.Name(), ch.Artist
	}

	ui.Column(c).Gap(2).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			col := t.TextMuted
			if st.Playing {
				col = t.Accent
			}
			ui.Box(c).Size(5, 5).Radius(3).Background(col)
			ui.Text(c, "NOW PLAYING").FontSize(9.5).LetterSpacing(1.4).TextColor(t.TextMuted)
		})

		ui.Text(c, title).FontSize(20).FontWeight(600).MaxLines(1).TextColor(t.Text)
		if artist != "" {
			ui.Text(c, artist).FontSize(11.5).MaxLines(1).TextColor(t.TextMuted)
		}
	})
}

// position is the scrubber and the two clocks.
func position(c *ui.Context, t *ui.Theme, m *Model, st player.State) {
	ui.Column(c).MarginY(0).Gap(2).Children(func() {
		SeekBar(c, progressOf(st.Position, st.Total), func(f float64) {
			if m.OnSeek != nil {
				m.OnSeek(f)
			}
		})
		ui.Row(c).Children(func() {
			ui.Text(c, Clock(st.Position)).FontSize(10).TextColor(t.TextMuted).Grow(1)
			ui.Text(c, "−"+Clock(st.Total-st.Position)).FontSize(10).TextColor(t.TextMuted)
		})
	})
}

// transport is previous, play, next.
func transport(c *ui.Context, m *Model, st player.State) {
	ui.Row(c).AlignItems(ui.Center).Justify(ui.Center).Gap(10).
		MarginY(10).Children(func() {
		StepButton(c, "prev", "Previous chapter", func() {
			if m.OnPrev != nil {
				m.OnPrev()
			}
		})
		PlayKey(c, st.Playing, func() {
			if m.OnPlay != nil {
				m.OnPlay()
			}
		})
		StepButton(c, "next", "Next chapter", func() {
			if m.OnNext != nil {
				m.OnNext()
			}
		})
	})
}

// chapterList is the tracklist: the only part that scrolls.
//
// Recording mode keeps the fixed chapter rows in a Scroll. Queue mode uses
// ui.List, a virtual list: it builds only the rows in view, so a folder of
// 207 songs costs the same as one of 20 (docs/ui/list.md).
//
// Height must be set explicitly. Grow(1) alone lets the list take its
// content's height, so it never overflows and never scrolls: the content has
// to be taller than a bounded box for a scroll offset to exist.
func chapterList(c *ui.Context, t *ui.Theme, m *Model, st player.State) {
	ui.Box(c).Width(288).Height(1).MarginY(8).Background(t.Border)

	if st.Queue {
		queueList(c, t, m, st)
		return
	}

	list := ui.Scroll(c).Width(288).Height(scrollHeight(c))
	list.TrackScroll(m.Scroll)

	list.Children(func() {
		for i := range mix.Chapters {
			chapterRow(c, t, m, st, i)
		}
	})
}

// queueList renders the folder's songs through ui.List. The ListState comes
// from the app: per docs/ui/views.md, widget state belongs in app state,
// and its fields are set before the list is built each frame.
func queueList(c *ui.Context, t *ui.Theme, m *Model, st player.State) {
	tracks := m.Player.Tracks()
	if m.List == nil {
		return
	}
	ls := m.List
	ls.Key = func(i int) any { return tracks[i].Path }
	ls.Label = func(i int) string {
		if tracks[i].Artist != "" {
			return tracks[i].Title + " — " + tracks[i].Artist
		}
		return tracks[i].Title
	}
	ls.Selected = &m.listSelection

	// Follow the playing song: when it changes (auto-advance, next), keep
	// it in view. The list answers ScrollIntoView on its next layout, and
	// Visible reads back where it landed, per docs/ui/list.md.
	if i := m.Player.CurrentTrack(); i != m.following {
		m.following = i
		if m.List != nil {
			m.List.ScrollIntoView(i)
		}
	}

	// The list reports a new choice through its element's Changed, so the
	// model hears of it once, whether it came from a click or the arrow keys.
	list := ui.List(c, ls, len(tracks), func(i int) {
		active := i == st.Chapter
		row := ui.Row(c).AlignItems(ui.Center).Height(40).Gap(8).PaddingX(6).Radius(8).
			Width(276)
		switch {
		case active:
			row.Background(fade(t.Accent, 0.10))
		case row.Hovered():
			row.Background(t.SurfaceHover)
		}
		row.Children(func() {
			col := t.TextMuted
			if active {
				col = t.Text
			}
			ui.Text(c, ls.Label(i)).FontSize(12.5).MaxLines(1).TextColor(col).Grow(1)
			if active && st.Playing {
				drawQueueBars(c, t, m)
			}
		})
	}).Width(288).Height(scrollHeight(c)).Gap(2).Padding(4, 2)

	if list.Changed() && ls.Selected != nil && *ls.Selected >= 0 && *ls.Selected < len(tracks) {
		m.Player.JumpTo(*ls.Selected)
	}
}

// drawQueueBars draws the three-bar playing marker inside the active row.
// It uses Loop, as the disc does, so the frames keep coming (loop_test.go
// pins the lesson: no manual repaint requests).
func drawQueueBars(c *ui.Context, t *ui.Theme, m *Model) {
	num := ui.Box(c).Size(20, 16)
	var phase float32
	if !c.Preferences().ReduceMotion {
		phase = num.Loop("bars", 1100*time.Millisecond, ui.Linear)
	}
	num.Draw(func(p *ui.Painter, r ui.Rect) {
		drawBars(p, r, t.Accent, phase)
	})
}

func chapterRow(c *ui.Context, t *ui.Theme, m *Model, st player.State, i int) {
	ch := mix.Chapters[i]
	active := i == st.Chapter

	el := ui.Row(c).AlignItems(ui.Center).Height(40).Gap(8).PaddingX(6).Radius(8)
	switch {
	case active:
		el.Background(fade(t.Accent, 0.10))
	case el.Hovered():
		el.Background(t.SurfaceHover)
	}

	el.Children(func() {
		num := ui.Box(c).Width(22)
		if active && st.Playing {
			// The bars come from Loop, the same way the disc does: it keeps
			// frames coming while the element is built, so they move on the
			// normal frame path. Deriving them from a repaint request left
			// them stepping whenever a repaint was not granted.
			var phase float32
			if !c.Preferences().ReduceMotion {
				phase = num.Loop("bars", 1100*time.Millisecond, ui.Linear)
			}
			num.Draw(func(p *ui.Painter, r ui.Rect) {
				drawBars(p, r, t.Accent, phase)
			})
		} else {
			ui.Text(c, fmt.Sprintf("%02d", i+1)).FontSize(9.5).TextColor(t.TextMuted)
		}

		titleCol := t.TextMuted
		if active {
			titleCol = t.Text
		}
		ui.Text(c, ch.Name()).FontSize(12.5).MaxLines(1).TextColor(titleCol).Grow(1)
		ui.Text(c, Clock(mix.Length(i, st.Total))).FontSize(9.5).TextColor(t.TextMuted)
	})

	if el.Clicked() {
		m.Player.JumpTo(i)
	}
	el.Label(ch.Label())
}

// drawBars draws the three-bar playing indicator.
//
// phase is the loop's progress, 0 to 1, and each bar is offset a third of the
// way round it, so they rise and fall out of step like a level meter. The
// height never reaches zero, which would read as a gap rather than a beat.
func drawBars(p *ui.Painter, r ui.Rect, col ui.Color, phase float32) {
	base := r.Y + r.H/2

	const barH, barW, gap = float32(8), float32(2.5), float32(5)
	for i := 0; i < 3; i++ {
		a := float64(phase)*2*math.Pi + float64(i)*2.1
		h := barH * float32(0.25+0.75*(0.5+0.5*math.Sin(a)))
		x := r.X + 4 + float32(i)*gap
		p.Fill(ui.Rect{X: x, Y: base - h/2, W: barW, H: h}, col, 1.2)
	}
}

// emptyState is what the popup shows before a recording is chosen. It says
// plainly that the app downloads nothing, because a lo-fi app that never
// plays could be mistaken for one that failed.
func emptyState(c *ui.Context, t *ui.Theme, m *Model) {
	ui.Column(c).Fill().Gap(12).PaddingY(56).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "No recording yet").FontSize(17).FontWeight(600)
		ui.Text(c, "Drift plays a mix that is already on this computer. It downloads nothing.").
			FontSize(12).TextColor(t.TextMuted).TextAlign(ui.Center).MaxWidth(240)
		if ui.PrimaryButton(c, "Choose your mix").Clicked() && m.Choose != nil {
			m.Choose()
		}
	})
}

// scrollHeight is the room the chapter list gets: what is left of the popup
// after the header, plate, title, times and transport. A fixed number,
// because a list that sizes to its content never overflows and never scrolls.
func scrollHeight(c *ui.Context) float32 {
	_, h := c.Size()
	n := h - 16 - 30 - 158 - 66 - 46 - 62 - 8
	if n < 88 {
		return 88
	}
	return n
}
