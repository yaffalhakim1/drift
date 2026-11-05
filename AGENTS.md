# AGENTS.md — drift

A lo-fi mix player. Two builds live here, and they share one tracklist.

| Path | What it is | State |
| --- | --- | --- |
| `index.html`, `styles.css`, `src/` | Browser version. Synthesizes its own audio. | Working, reference for the design |
| `desktop/` | MyGo 0.7.4 native tray app. No webview. | Working, in active development |

The desktop app is the product. The browser version is the design reference and
the place where a look is tried first, because it can be screenshotted.

## Read this first

Load the `mygo-native-ui` skill before touching `desktop/`. It is not optional.
MyGo ships a full component library and hand-rolling a control is the single
easiest way to produce inconsistent UI here. The skill's inventory names every
component; the docs are local:

```
C:\Users\yafit\go\pkg\mod\github.com\egoist\mygo@v0.7.4\docs\ui\
```

`go doc github.com/egoist/mygo/ui` works too.

## Hard rules

1. **Never hand-roll a control that exists.** Use `ButtonBase`, `SliderBase`,
   `SegmentedBase`, `SelectBase`, `SwitchBase` and the rest. Style their parts;
   do not rebuild their behaviour. The seek bar, the play key, the step buttons
   and the appearance switch are all built on bases for exactly this reason.
2. **Do not invent API.** Every MyGo call in this repo was checked against the
   module source first. A guess costs a build cycle and, worse, a wrong claim.
3. **The recording is not in the repo.** It is the user's file, streamed from
   disk. Nothing that ships may contain audio.
4. **Attribute the mix.** The 28 chapters are other artists' work. Keep the
   artist names in the data and never present the mix as ours.

## Layout of the code

```
desktop/
  main.go                   tray, flyout, file picker, theme persistence
  mygo.json                 name, identifier, version
  internal/mix/tracks.go    the 28 chapters; no audio
  internal/player/          stream the file, seek, transport, chapter state
  internal/view/
    view.go                 the screen: header, plate, meta, seek, transport, list
    plate.go                the record, drawn with the painter
    controls.go             play key, step buttons, seek bar: MyGo bases
    theme.go                Apple blue accent, both appearances
```

The player outlives the popup, so music continues when the window closes. That
is the whole reason audio lives in Go and not in a page.

## The design

- One accent: Apple blue. `#0A84FF` on dark, `#0A6FD6` on light. Apple's own
  `#007AFF` is 3.79:1 on the light surface, under the 4.5:1 floor, so light uses
  the darker variant. Both were computed, not estimated.
- One object: a record, 148 px, turning 12 s per revolution.
- The label print never turns. A rotated title is unreadable in a still frame.
- Type is the hierarchy. Title 20 px semibold, artist 11.5 px muted, kicker
  9.5 px tracked.
- The play key is the brightest thing on the card in both themes.

## Traps in MyGo, learned the hard way

These cost hours. Each one is now covered by a test.

**`ui.Themed` must be given `.Fill()`.** It builds a plain column. Without Fill
that column takes its content's height, so the chapter list measured 1120 px in
a 585 px window and never scrolled. Test: `screen_test.go`.

**`ui.Scroll(c)` needs `.Children(func(){...})`.** Building rows after the call
makes them siblings of the list, not its content, so the list stays empty.

**Use `Element.Loop` for animation, not painter repaint requests.**
`Loop(key, period, ease)` returns 0..1 and keeps frames coming while the element
is built. Manual `AnimationFrameRate` from inside a Draw callback left the frame
in a state the engine rejected, so the popup rebuilt every frame: 259 text
layouts and 18,000 allocations per frame, and the bars beside the song name
stepped. Test: `loop_test.go`.

**The wheel reports `dy` positive downwards.** Negative scrolls nothing, which
made three of my scroll tests pass while proving nothing. Test: `screen_test.go`.

**`ui.RGBA` takes alpha as 0..1**, not 0..255. `Painter.Transform(t, ox, oy)`
rotates around `(ox, oy)`, not around a box corner.

**Tester's `Frame()` settles up to 20 passes** while anything asks for a frame.
A view count of 20 per `Frame()` with an animation is the harness, not a bug.

## Testing

```
cd desktop
go test ./...                                    # unit tests
$env:DRIFT_MIX="<path to the user's mp3>"; go test ./...   # with the real file
$env:DRIFT_PREVIEW=1; go test ./internal/view -run TestDump  # writes previews
```

The tests that matter build the **real screen** with the **real recording** and
drive it: `TestScreenScrolls` wheels the list, `TestStreamRealRecording` seeks
and checks the chapter. A test of an isolated widget is not proof.

Never claim a fix from a test that does not exercise the thing that was broken.
I did that twice here.

## Running and building

```
cd desktop
go build -o drift-dev.exe . && .\drift-dev.exe    # runs in the tray
$env:MYGO_FRAME_STATS="1"                          # logs frames over 8 ms
go tool mygo build                                 # installer
```

The app has no window until the tray icon is clicked. `MainWindowHandle` being 0
is correct, not a failure.

## Style

- Comments say why, never what. Where MyGo behaves surprisingly, the comment
  says so, because the next reader will otherwise "fix" it.
- Short sentences. Name the actor. No idioms.
- Every appearance choice with a number in it should say where the number came
  from: measured, computed, or taken from the design.
