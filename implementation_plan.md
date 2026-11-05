# Plan: drift as a native tray player

Status: waiting for approval. No Go code yet.
Owner: design engineer.
Platform: Windows first. macOS and Linux after.

## 1. Goal

Turn `drift` into a native desktop app that lives in the system tray.

- Click the tray icon. A small popup opens under the icon.
- The popup plays lo-fi music.
- Close the popup. The music keeps playing.
- No webview. All pixels are drawn by MyGo.
- No browser tab. No full window. No taskbar entry.

## 2. Why the architecture must change

The current app is one HTML page. The music lives in the page's Web Audio graph.

A MyGo flyout is destroyed when it closes. A native window has no webview at all.
So the page cannot own the audio any more.

Decision: the music engine moves into Go. The popup becomes a view of that engine.

This is simpler than it sounds. It is also the correct shape for a tray app:
one long-lived engine, one short-lived window onto it.

## 3. What I checked in MyGo

Read from the local clone at `Y:\Learn\Go\mygo` and from mygo.egoist.dev.

| Need | MyGo answer | Confirmed |
| --- | --- | --- |
| Popup pinned to the tray icon | `NewFlyout(FlyoutOptions{Tray, Placement: PlacementBottom, Focusable: true})` | `flyout.go:51` |
| Keep app alive with no window | `App.OnWindowAllClosed(func(){})` | `examples/menubar-flyout/main.go:143` |
| Tray click toggles popup | `tray.OnClick(toggle)` | same file, line 101 |
| Right-click menu | `tray.OnRightClick` + `PopUpMenu` | same file, lines 102-110 |
| Custom drawing | `Painter.FillPath`, `StrokePath`, `Path.Circle`, `FillGradient`, `Shadow` | `ui/paint.go` |
| Motion | `Element.Spring(key, target)`, `AnimationFrame` | `ui/motion.go` |
| System light/dark | `mygo.Theme.IsDark`, `Theme.OnUpdated` | `docs/native` |
| Audio playback | `audio.NewSound`, `Play`, `Pause`, `Seek`, `Position`, `OnEnd`, `Loop` | `audio/audio.go` |
| Audio streaming | `audio.OpenStream(io.ReadSeeker)` with a custom `Decoder` | `audio/audio.go` |

Two constraints found:

1. `FitContent` does not size native UI. A flyout showing native UI needs an explicit `Width` and `Height`.
2. MyGo is on `v0.7.4`, the newest release. The local clone has been removed, so the plan pins the module at `v0.7.4` and every call is checked against that module's source, not memory and not an older copy.

## 3a. The content model: one mix, twenty-eight chapters

The tracklist has timestamps that run end to end, `[00:00]` through `[58:06]`.
That is one continuous recording, cut into chapters, not twenty-eight songs.

The app therefore stores:

- **One recording.** 59 minutes 16 seconds.
- **Twenty-eight chapter markers**, each with an offset, an artist and a title.
- The chapter at the playhead decides what the title, artist and highlight show.

This is how mix players work, and it is better than fake separate tracks:
seeking is one continuous position, `next` jumps to the next marker, and
`previous` first rewinds to the start of the chapter, then goes back one.

What the app does **not** ship, and what the repository does **not** contain,
is the audio. The recording belongs to its artists, and a 59 minute audio file
does not belong in a git repository in any case. The app plays a file the user
supplies, from wherever the user already keeps it:

```
C:\Users\<you>\Music\whatever-you-called-it.mp3    (outside the project)
```

The app finds it in this order, first hit wins:

1. The path the user picked last time, kept in the app's data folder.
2. `mix.mp3`, `mix.ogg` or `mix.wav` beside the executable, for a user who
   prefers to drop a file next to the app.

If there is no file, the popup still opens. The player is disabled, the
tracklist is shown greyed out, and one line explains it with a single button:
"Choose your mix file". One click, a native file dialog, and it plays.
No silent stand-in pretending to be the tracklist.

Chapters come from `mockup/src/mix.js`, exported once to `desktop/internal/mix/tracks.go`
as offsets, artist and title. No audio, just numbers and names.

Formats, checked against the package source, not assumed:

| Format | Plays | What it needs |
| --- | --- | --- |
| WAV | yes | nothing, the package registers it itself |
| MP3 | yes | one blank import of `audio/mp3` |
| Ogg Vorbis | yes | one blank import of `audio/vorbis` |
| M4A / AAC | no | no decoder exists; the file must be converted |

So an MP3 works as-is. An `.m4a` from Apple Music or a download is the only
case that needs converting, and the app says so plainly instead of failing.

## 4. Audio design

The recording is streamed from disk, never held in memory. A 59 minute stereo
mix as float32 samples would be about 1.2 GB, so `audio.OpenStream` is the
right call, not `audio.NewSound`: it decodes as it plays and seeks inside the
file.

`mygo/audio` has no filters, no effects and no oscillators. It plays samples.
So the synthesized fallback still needs our own code.

### With a mix file: stream it

```
f, _ := os.Open(path)
p, err := audio.OpenStream(f)          // decodes as it plays, seeks in place
p.SetVolume(0.8).Play()
```

No synthesis, no processing. The file is the sound. `p.Position()` drives the
scrubber, `p.Seek(d)` jumps, and `p.OnEnd` moves to the end of the queue.
The file stays open while it plays and closes with the player.

### Without a mix file: the synthesized fallback

Not shown to the user as an equal option. It exists so the app is still
playable if the file cannot be read, and it is the same synth that already
sounds right in the web build, ported.

Implement `audio.Decoder` and hand it to `audio.OpenStream`.
The decoder synthesizes samples on demand.

```
type Decoder interface {
    SampleRate() int
    Channels() int
    Len() int64
    Read(buf []float32) (int, error)
    SetPosition(frame int64) error
    Close() error
}
```

Why this over pre-rendering to a buffer:

| | Streaming decoder | Pre-rendered buffer |
| --- | --- | --- |
| Memory | a few MB, fixed | ~18 MB per track at 44.1 kHz |
| Seek | free, just set the index | free, it is an index |
| Startup | instant | ~0.3 s per track |
| Cost | small work per audio block | none during play |


Memory decides it. Five tracks pre-rendered is ~90 MB. Not acceptable for a tray app.

### Signal chain, per block of 512 frames

1. **Sequencer.** 16 steps per bar, at each track's BPM, with swing. Tracks carry a chord progression, a drum grid, a bass grid, a melody grid and a seed.
2. **Voices, all preallocated.**
   - Rhodes-style keys: two sines, one modulated by the other, with a soft attack and a long decay.
   - Bass: sine plus a quiet triangle one octave down.
   - Bells: sine with a fast decay.
   - Drums: kick as a pitch-swept sine, snare and hats from a noise source and a filter.
3. **Character, applied to the mix.**
   - Saturation: `tanh`.
   - Low-pass: one-pole, around 7 kHz, to take the top off.
   - Wow and flutter: a slow sine on pitch, shared by all voices.
   - Hiss: a quiet band-passed noise bed.
   - Crackle: random short noise bursts.
   - Small room: a short feedback delay, not convolution, so it costs almost nothing.
4. **Limiter.** Soft clip to ±0.9 so nothing distorts.

All buffers are allocated once, at construction. `Read` must not allocate.

### Seek and loop

`SetPosition` resets the sequencer to the step that contains that frame, clears the delay lines and ramps the gain in over 5 ms. A pause also ramps out, so no click.
Loop is handled by seeking to zero at the end of the track, not by `SetLoop`, so the tail fades instead of jumping.

### Verification, before any UI exists

The engine is pure Go with no window, so it is tested directly:

- `go test` renders 10 seconds and asserts the signal is not silent and does not clip.
- It asserts no gap longer than 20 ms inside a bar.
- It asserts seek lands within one frame of the target.
- It asserts two runs with the same seed are identical, so the sound is stable.

This is an E2E-style check of the one part that cannot be verified by looking.

## 5. Native interface design

The plate replaces the cassette. A cassette is ~40 paths and does not scale down to a 190 px popup. A record plate is one disc, a set of grooves, a label and a spindle, and it carries the same idea: warm physical media, tape-era colour.

### Layout, 320 × 585 DIP

Sized to the tray popups already on this machine, measured from a screenshot:
Pulse is about 320 × 585 DIP. The app should not be bigger than the popups the
user already lives with, so this is the hard ceiling, not a preference.

```
┌────────────────────────────┐
│ drift                 ◐    │  header, 30
│                            │
│        ╭────────╮          │  plate, 168
│       │ grooves  │         │  label turns, print stays level
│       │  ● snow  │         │
│        ╰────────╯          │
│                            │
│ ● NOW PLAYING              │  kicker, 10
│ Snowman                    │  title, 20
│ WYS                        │  artist, 11
│                            │
│ ─────────●──────────────   │  seek, 3 px rail
│ 0:42              −0:42    │  times, 10
│                            │
│     ⏮     ▶     ⏭         │  transport, 42 play key
│                            │
│ ─────────────────────────  │
│ 01  Snowman        WYS 3:10│  rows, 40 px, scroll
│ 02  Cotton Cloud  Fatb 3:13│
│ 03  the places…   2:16     │
│ 04  wool gloves   2:44     │
│ 05  I'm sorry     2:15     │
└────────────────────────────┘
```

Rows are 40 px, not 48: five chapters visible beats three, and a chapter list
is what a mix needs.

### Plate drawing

Drawn with the painter, in this order:

1. Outer shadow, a soft drop, 12 px blur.
2. Disc: radial gradient, near-black to slightly lighter at the centre.
3. Grooves: 20 faint circles, 0.7 to 1.1 px, alpha falling toward the rim, so
   the disc reads as a surface rather than a barcode.
4. Sheen: a fixed linear gradient across the upper left, so the disc is glossy.
5. Label: a cream disc with a progress ring on its edge.
6. Print: title over artist, centred, clipped to a chord across the label, so
   long titles shrink and then shorten instead of escaping the circle.
7. Spindle: a small dark hole at the centre.

Rotation: the disc turns, 30 degrees a second. The label print **stays level**,
because a rotated title is unreadable in a still frame. That is the one
departure from a real record, and it is the right one.

The label edge carries a progress ring, so the plate shows position even at
168 px where the seek bar is thin.


### Colour

Two palettes, taken from the current CSS so the desktop app matches the web version.

| Role | Dark | Light |
| --- | --- | --- |
| background | `#08080a` | `#f1ece3` |
| surface | `#101013` | `#fbf8f2` |
| text | `#f3efe7` | `#1b1814` |
| muted | `#a09a90` | `#554f48` |
| dim | `#8b857b` | `#6b6459` |
| accent | `#e8b487` | `#9c5720` |
| label | `#f2e9d8` | `#fdfaf3` |

Every pair already passes WCAG AA. Verified with computed contrast ratios in the web build.

### Motion

- Press: scale to 0.97, `ui.Snappy`.
- Play key: 46 px, the brightest object on screen. A gradient fill, an inset highlight and a drop shadow, as in the web build.
- Title change: 160 ms crossfade with a 2 px blur, on a `ui.Smooth` channel.
- Reels and equaliser bars: `AnimationFrame` while playing only.
- Reduced motion: `mygo.Theme.Preferences().ReduceMotion` stops rotation. Opacity changes stay.

## 6. Interaction

| Action | Result |
| --- | --- |
| Left-click tray icon | open the popup, or close it if open |
| Right-click tray icon | menu: Play/Pause, Next, Quit |
| Space | play or pause |
| Left / Right | seek 5 s |
| Shift + Left / Right | previous or next track |
| Escape | close the popup, music keeps playing |
| Click outside the popup | close only |

The popup is `Focusable`, so it closes on an outside click, like a menu.
Closing it never stops playback, because playback is not owned by the window.

## 7. Files

New module at `Y:\Learn\Go\youtube-playlist\desktop`. The web version stays untouched.

```
desktop/
  mygo.json                      app name, identifier, version
  main.go                        tray, flyout, lifecycle, shortcuts
  resources/
    icon.png                     app icon, 1024
    tray.png                     tray icon, 32, template on macOS
  internal/lofi/
    decoder.go                   audio.Decoder, block rendering, seek, loop
    sequencer.go                 steps, bars, swing, chord table
    voices.go                    keys, bass, bells, drums
    character.go                 saturation, low-pass, wow, hiss, crackle, delay
    noise.go                     deterministic noise, so a seed is a sound
    tracks.go                    the five tracks, same data as the web build
    decoder_test.go              signal, gaps, seek, repeatability
  internal/player/
    player.go                    transport state, queue, volume, events
    player_test.go               transitions, next and previous wrap
  internal/view/
    view.go                      the screen: header, plate, meta, seek, transport, queue
    plate.go                     the record plate drawing
    controls.go                  play key, ghost buttons, seek bar, queue rows
    theme.go                     two palettes, derived from ui.Theme
    format.go                    mm:ss and BPM formatting
    view_test.go                 ui.Tester: click play changes state
```

## 8. Phases

Each phase ends at something runnable or testable. No phase starts before the one before it passes.

1. **Scaffold.** `mygo init -template native desktop`. Pin the module. Confirm `go tool mygo dev` runs an empty window.
2. **Audio engine.** Write `internal/lofi`. `go test` proves signal, no gaps, seek and repeatability. Nothing visual yet.
3. **Player state.** Transport, queue, wrap-around, volume. Unit tests.
4. **Plate.** Draw the disc, grooves, sheen and label. Static first, then rotation.
5. **View assembly.** Header, meta, seek, transport, queue. Wire the controls to `internal/player`.
6. **Tray and flyout.** Icon, click toggle, right-click menu, popup close without stopping audio.
7. **Theme and motion.** Light and dark follow the system. Motion values as in section 5.
8. **Harden.** Reduced motion, high contrast, 150 percent display scaling, long titles, one-letter titles, 300 BPM.
9. **Build.** `go tool mygo build` and run the installed app.

## 9. Risks

| Risk | Mitigation |
| --- | --- |
| Synth on the audio thread is too slow and crackles | preallocate everything, keep voices cheap, measure with a bench test; fall back to a pre-rendered 30 s loop only if it fails |
| MyGo 0.6.3 CLI differs from the 0.7.4 docs | check every call against the pinned module before use; upgrade the module if a call is missing |
| A tray app cannot quit cleanly | `OnBeforeQuit` destroys the tray and closes the audio player |
| Flyout does not fit native UI to content | fixed `Width` and `Height`, checked at 150 percent scaling |
| Seek clicks because voices restart mid-cycle | 5 ms fade out, reset, 5 ms fade in |

## 10. Out of scope for the first build

- Windows media controls and media keys. The API exists (`NowPlaying`), so this is a fast follow-up, not a rewrite.
- Auto-update and signing.
- macOS and Linux packaging. The code is portable, but each platform's tray behaviour needs its own pass.
- Lyrics, favourites, search.

## 11. Approval needed

1. This plan, as written.
2. The streaming decoder instead of pre-rendered audio.
3. Windows first, one platform.
4. New `desktop/` folder, web version untouched.
