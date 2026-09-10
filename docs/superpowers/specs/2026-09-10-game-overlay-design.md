# Game overlay (beta) — design

Date: 2026-09-10
Status: approved, not implemented

## Problem

Giant Cursor enlarges the pointer by calling `SetSystemCursor` on the standard
`OCR_*` system cursor ids. That only affects applications that ask Windows for a
standard cursor. Games such as World of Warcraft ship their own cursor artwork
and set it directly, so there is no system entry to substitute and the pointer
stays small inside the game.

Hooking the game process to intercept its cursor calls would work and is
forbidden: World of Warcraft runs Warden, and injection is exactly what
anti-cheat systems look for. No part of this design touches another process.

The remaining option is to draw our own always-on-top window that follows the
pointer. The game keeps drawing its cursor; we annotate the screen next to it.

## Goals

- Make the pointer findable inside windowed games, without touching the game.
- Offer two visual treatments and let the user pick.
- Ship it off by default and clearly labelled beta.

## Non-goals

- Exclusive fullscreen. A layered window does not composite over an exclusive
  swapchain. Windowed and borderless windowed only. This is documented, not
  worked around.
- Hiding the game's own cursor. Impossible without injection, which is
  permanently out of scope.
- Persistent game mode (overlay stays on while a game has focus, no shake
  needed). Deferred until real play testing shows the shake gesture is
  impractical mid-combat.

## Architecture

### The existing seam carries the feature

`cursor.Enlarger` (`Enlarge`, `Restore`) stays unchanged. The overlay is a
second implementation behind the same port, and both run together: the system
cursor swap keeps working on the desktop while the overlay covers the game.

A new `cursor.Multi` composes them. It fans `Enlarge` and `Restore` out to each
member in order and returns the first error, after attempting all of them, so
one failing effector never leaves the other stuck.

### Position updates: an optional port extension

`Enlarge` and `Restore` are edge-triggered, but a window that follows the
pointer needs a position on every sample. Rather than widen `Enlarger` and break
its implementations, add an optional interface next to it:

```go
// Tracker is an optional Enlarger extension for effects that follow the
// pointer while enlarged.
type Tracker interface {
    Track(x, y int)
}
```

It takes plain ints so `internal/cursor` gains no dependency on
`internal/shake`.

`internal/app` type-asserts its `Enlarger` to `Tracker` once at construction
time, not per sample, and calls `Track` on every `Step` while the state is
`StateBig`. `cursor.Multi` implements `Tracker` and forwards to whichever
members implement it.

Effectors that do not implement `Tracker` are unaffected. The existing Win32
cursor effector is one of them.

### Rendering: paint once, move often

The bitmap is built once per activation, not per frame:

- On `Enlarge`: render the shape, push it to the window with
  `UpdateLayeredWindow`, then show the window.
- On `Track`: `SetWindowPos` to centre the window on the pointer. Nothing is
  repainted.
- On `Restore`: hide the window. The bitmap is kept for reuse and only
  rebuilt when the shape or size changes.

The pointer position already arrives at 125 Hz (an 8 ms poll interval in
`cmd/giant-cursor/main.go`), which is faster than typical display refresh.
Moving a window at that rate is cheap; repainting it would not be.

Window extended styles: layered, transparent (click-through), topmost, no
activate, and tool window. Click-through matters most: the overlay must never
swallow a click meant for the game.

### The painter is the pluggable part

The expensive work is the window, not the shape. The shape sits behind a small
interface in `internal/overlay`:

```go
// Painter renders the overlay bitmap for a given cursor size in pixels.
type Painter interface {
    // Size returns the window edge length in pixels for a cursor size.
    Size(cursorSize int) int
    // Draw renders the overlay art, in premultiplied alpha.
    Draw(cursorSize int) *image.RGBA
}
```

Two painters:

- **Halo** draws a ring centred on the pointer. It annotates rather than
  replaces, so it does not compete with the game's own cursor, and a soft wide
  shape hides any positional lag.
- **Pointer** draws an enlarged arrow at the pointer hotspot, reusing the
  existing high-resolution artwork. The user will see two cursors: ours and the
  game's. That is expected behaviour for this mode, not a defect.

The halo's starting geometry, to be tuned against real play: outer diameter
2.5x the enlarged cursor size, ring thickness 12% of that diameter, drawn as a
bright ring with a dark outline on both edges so it stays visible against both
light and dark scenes. It scales with the same factor the user already
configures for cursor size, so one setting drives both effects.

To feed the pointer painter without moving files, `internal/cursor` exports a
render helper over its existing artwork:

```go
// ArrowImage renders the crisp arrow artwork at the given cursor size.
func ArrowImage(size int) image.Image
```

`internal/cursor/art.go` has no build constraint, so this keeps the painters
buildable and testable on any platform.

### Configuration

A new enum in `internal/config`, following the existing `Sensitivity` and
`cursor.Style` pattern:

```go
type OverlayMode string

const (
    OverlayOff     OverlayMode = "off"
    OverlayHalo    OverlayMode = "halo"
    OverlayPointer OverlayMode = "pointer"
)
```

Unknown values fall back to `OverlayOff`, matching how `ShakeConfig` already
handles unknown sensitivities.

Default is `off`. The feature only helps inside games, it changes what appears
on screen, and it is beta. The user turns it on deliberately.

When the mode is `off`, no overlay window is created at all. The cost when
unused is zero.

Switching mode at runtime takes effect on the next activation, never mid-shake.
Selecting `off` while the overlay is visible hides and destroys the window
immediately; switching between `halo` and `pointer` swaps the painter and
discards the cached bitmap, which is rebuilt on the next `Enlarge`.

### Tray menu

A new submenu following the existing style and sensitivity pattern: an
`idOverlayBase` id range, an `Overlays []string` list of values, a
`CurrentOverlay` getter and an `OnOverlay(value string)` callback, dispatched in
the same `case id >= ... && id < ...` chain in `internal/lifecycle/tray_windows.go`.

The submenu label carries the beta marker so it is visible in the product, not
only in the docs. Labels go through `internal/i18n` like every other menu
string, with entries for the submenu title and the three option names.

### Threading

Win32 windows are thread-affine. The overlay follows the pattern already
established by `internal/lifecycle/tray_windows.go`: its own goroutine with
`runtime.LockOSThread`, its own window class, and its own message loop.

It does not share the tray's loop. Separate loops keep the two lifetimes
independent, and the overlay can be created and destroyed when the mode changes
without disturbing the tray.

`Track` is called from the polling goroutine and only issues `SetWindowPos`,
which is safe against a window owned by another thread. `UpdateLayeredWindow`
runs on the overlay's own thread, driven by a message posted from `Enlarge`.

## Error handling

The overlay is an enhancement, never a prerequisite. If the window cannot be
created, the failure is reported once through the existing debug channel
(`GIANTCURSOR_DEBUG`) and the mode falls back to `off` for the session. The
system cursor effector keeps working on its own. The app never exits because
the overlay failed.

## Testing

Strict TDD applies: tests first for everything below.

- **Painters** are pure functions of a size. Given a cursor size they return an
  image, so they are tested the way `internal/cursor/art_test.go` already tests
  artwork: dimensions, hotspot placement, and that the shape is actually drawn
  (non-transparent pixels where expected, transparent where not).
- **`cursor.Multi`** is tested with fakes: both members receive `Enlarge` and
  `Restore`, all members are attempted when one fails, the first error is
  returned, and `Track` reaches only the members that implement `Tracker`.
- **`internal/app` wiring** is tested with a fake effector implementing both
  interfaces: `Track` is called on samples while big, not called while normal,
  and an effector without `Tracker` is driven without panicking.
- **Config** parsing of the three values plus an unknown one, mirroring the
  existing config tests.
- **The Win32 window** stays deliberately thin and untested, exactly like the
  existing cursor adapter. Any logic worth testing belongs in a painter.

## Risks

- **Anti-cheat.** Low but not zero. It stays low only because the overlay is an
  ordinary top-level window that never injects, never hooks the game, and never
  reads its memory. That constraint is not revisitable.
- **Two visible cursors** in pointer mode. Named in the UI and in the README so
  the user knows which mode to pick.
- **Exclusive fullscreen** silently shows nothing. Documented as a requirement
  to play in windowed mode.
