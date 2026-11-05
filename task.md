# Task: drift as a native tray app

Status: the app runs. Waiting on hands-on testing in the tray.

## Where it is

`desktop/` is a MyGo 0.7.4 native app. No webview, no HTML. Everything is drawn
by MyGo's GPU painter.

```
desktop/
  main.go                     tray, flyout, tray icon, file picker
  mygo.json                   name, identifier, version
  internal/mix/tracks.go      the 28 chapters, no audio
  internal/player/player.go   stream the recording, seek, transport
  internal/view/plate.go      the record, drawn
  internal/view/controls.go   play key, step buttons, seek bar
  internal/view/view.go       the screen
  internal/view/theme.go      Apple blue accent, both appearances
```

## What is built

- Tray icon, click to open or close the popup, right-click menu.
- Popup 320 × 585 DIP, the size limit you gave.
- The record plate: disc, grooves, sheen, cream label, blue progress ring.
  The disc turns; the print stays level so it can be read.
- 28 chapters from your tracklist, with lengths computed from the offsets.
- Streaming playback from a file you choose, remembered between runs.
- Apple blue accent: #0A84FF on dark, #0A6FD6 on light for contrast.

## Verified

- `go vet` clean, `gofmt` clean, all tests pass.
- Your file streams: 61:14 total, seek to 36:08 landed at 36:08, chapter 17.
- Every one of the 28 timestamps sits in a quiet gap in your recording,
  11 to 74 dB below the music around it.
- The plate renders: label light, vinyl dark, in both themes.
- The app runs: 29 MB, 7 threads, no window, idle CPU.

## Not verified, needs your hands

- That the tray icon is visible and clickable.
- That the popup opens under the icon at the right size.
- That music plays and the disc turns.

## Known gaps

- The theme switch in the header does nothing yet. The popup follows the
  system appearance; a manual override is a small follow-up.
- No media keys or Windows media controls yet.
- macOS and Linux are unexercised. The code is portable, the tray behaviour
  is not identical.

## To build a real app

```
cd desktop
go tool mygo build
```

`mygo build` packages an installer. `mygo dev` hot-reloads.
