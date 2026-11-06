# Folder mode done

## What shipped
- Queue mode in `internal/player/player.go`: `OpenQueue`, `FolderTracks`,
  `Tracks`, `CurrentTrack`, auto-advance on `OnEnd`, wrap-around next/prev,
  filename → title/artist parsing.
- "Choose folder of music…" in the tray menu and remembered folder
  (`last-folder`) in `main.go`; a remembered folder beats a remembered
  single recording at startup.
- The view renders the queue through `ui.List` (virtual), titles and
  artists come from the parsed names; recording mode unchanged.

## Verified
- `go test ./...` all green, including the new queue tests.
- Real folder: `DRIFT_FOLDER=downloads/sheeran-mix` — 106 tracks,
  first "Thinking Out Loud" by "Ed Sheeran", jump to last worked.
- Real screen in queue mode: first row laid out at y=245 inside the
  585 window; `ScrollIntoView` to the end moved the virtual window
  (0..3 → 98..101) and built the rows.
- Frame stats with the real folder: first frame 19 ms (87 text layouts),
  subsequent frames cheap — the virtual list does what the docs promise.
- `gofmt` clean, `go vet` clean.

## Notes for the morning
- The downloader was still running during tests; the track count changed
  between runs (74 → 106). One duplicate pair exists (016 has both .mp3
  and .webm); FolderTracks only lists supported extensions.
- `parseQueueName` keeps " ft. X" in the title when yt-dlp put it after
  the parenthetical; it reads fine on screen.
- Wheel scrolling of the queue is glide-smoothed by MyGo; the test
  proves the window moves via `ScrollIntoView` + `Visible` instead.
- Tray menu still says "Next chapter"; in queue mode it means next song.
