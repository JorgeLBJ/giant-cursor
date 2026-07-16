# Giant Cursor — Development Record

**Status:** v1.0.0 shipped
**Design:** [`../specs/2026-07-16-giant-cursor-design.md`](../specs/2026-07-16-giant-cursor-design.md)

This is the record of how Giant Cursor was actually built: the order, what each
piece is for, and the lessons that changed the design. It replaces the original
step-by-step plan, which described an app that no longer exists (global hotkey,
native cursor sizing, no tray). Those pivots are documented in the design doc
under *Rejected approaches*.

---

## Build order

Pure domain first, Windows last. Everything below `internal/` that has no
`_windows.go` suffix runs and tests on any OS.

| # | Piece | Kind |
|---|---|---|
| 1 | `internal/shake` — types + reversal detection + hold | **TDD** |
| 2 | `internal/config` — sensitivity presets | **TDD** |
| 3 | `internal/cursor`, `internal/input` — ports | interfaces |
| 4 | `internal/app` — detector → enlarger state machine | **TDD** (fake enlarger) |
| 5 | `internal/cursor/cursor_windows.go` — `SetSystemCursor` adapter | manual |
| 6 | `internal/input/poller_windows.go` — `GetCursorPos` adapter | manual |
| 7 | `internal/lifecycle` — single instance, tray, cleanup | manual |
| 8 | `internal/control` — thread-safe live settings | **TDD** (fake factory) |
| 9 | `internal/i18n` — English/Spanish labels | **TDD** |
| 10 | `internal/cursor/art.go` — artwork scaling + hotspot | **TDD** |
| 11 | `cmd/giant-cursor` — flags, config, wiring | end-to-end |
| 12 | `cmd/genicon`, `cmd/genarrow` — asset generators | dev tools |
| 13 | Installer, README, LICENSE, release CI | packaging |

## Testing strategy

- **Pure logic is test-first.** `shake` is driven with synthetic
  `(position, timestamp)` sequences: a real shake triggers; straight movement,
  sub-noise drift, and smooth movement after a shake do not; continued shaking
  re-arms the hold.
- **Win32 adapters are not unit-tested.** They are thin syscall glue, verified by
  building and running.
- **Everything visual is verified by rendering it and looking at it.** The
  artwork pipeline is checked by opening the generated PNGs, not by asserting
  pixels — see the lessons below.

Run the suite with `go test ./...`.

## Lessons that changed the code

Each of these came from real use, not from planning.

1. **Shrink on *shake* activity, not on movement.** The first build kept the
   cursor enlarged while the mouse moved at all, so it stayed huge during
   ordinary use. Only continued *shaking* re-arms the hold now.
2. **Never use `Ctrl+Alt` for a global hotkey.** It is AltGr on Spanish
   layouts — the quit hotkey ate the `@` key. The tray replaced it.
3. **Windows only ships 32px cursors.** Any approach that enlarges them is
   blurry by construction. Crispness required bringing our own high-resolution
   artwork and scaling it *down*.
4. **Don't hand-draw art you already have.** Several rounds were spent tuning a
   polygon to match a reference image that could simply be used directly.
5. **Look at the image.** A cursor was "verified" by sampling pixels while it was
   rendered on a white background — where white fill and transparency are
   indistinguishable. Render on a contrasting background and look.
6. **Rebuild before asking someone to test.** `go build ./...` compiles; it does
   not refresh `giant-cursor.exe`. One test round was spent on a stale binary.

## Adding a cursor style

No code required beyond one map entry:

1. Put the art in `assets/cursor-raw/<name>.png` — white shape, thick black
   outline, on a blue background.
2. Add `<name>` to `cursors` in `cmd/genarrow/main.go`.
3. Add `ocrXxx: <name>Art` to `crispArt` in `internal/cursor/cursor_windows.go`
   and the matching `//go:embed` in `internal/cursor/art.go`.
4. `go run ./cmd/genarrow && go build -o giant-cursor.exe ./cmd/giant-cursor`

The style suits shapes with volume (pointer, hand). It does **not** suit thin
shapes such as the text caret — see the design doc.

## Releasing

```bash
git tag v1.2.3 && git push --tags
```

CI tests, builds the portable exe and the Inno Setup installer, and publishes the
GitHub release.
