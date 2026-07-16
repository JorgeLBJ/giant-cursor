# Giant Cursor — Design

**Status:** Implemented (v1.0.0)
**Platform:** Windows 10/11, amd64
**Language:** Go 1.26, no CGo — one self-contained `.exe`

This document describes what Giant Cursor actually is and why it is built this
way, including the approaches that were tried and rejected. It is kept in sync
with the code; the user-facing documentation is [`README.md`](../../../README.md).

---

## 1. Problem

People with low vision routinely lose track of the mouse pointer. macOS solves
this by magnifying the pointer when you shake the mouse. Windows has no
equivalent, and PowerToys "Find My Mouse" is unsatisfactory:

- It draws a full-screen spotlight overlay, which forces whole-screen
  composition.
- It relies on a low-level global mouse hook (`WH_MOUSE_LL`), which Windows can
  starve when the system is busy — so it intermittently stops responding.

## 2. Goals

- Shaking the mouse enlarges the cursor; it returns to normal on its own.
- Negligible CPU; reliable even with many apps open.
- Never leaves the cursor stuck enlarged, even after a crash or forced kill.
- Configurable from a tray icon, in English or Spanish.
- Publicly distributable: installer, portable exe, MIT licensed.

### Non-goals

- No spotlight/halo effect.
- No settings window (the tray menu is the whole UI).
- No support for cursors that applications draw themselves.

## 3. Core decisions

### 3.1 Detection by polling, not by hook

A dedicated goroutine polls `GetCursorPos` every ~8 ms (~120 Hz). Polling cannot
be starved or detached the way a low-level hook can, which is the root cause of
PowerToys' unreliability. CPU cost is negligible.

### 3.2 Enlargement by `SetSystemCursor`

On shake, the standard system cursors are replaced with enlarged ones via
`SetSystemCursor`. There is **no overlay and no per-frame drawing** — the OS
renders the (bigger) cursor as it always does. Restore reloads the user's cursor
scheme with `SystemParametersInfo(SPI_SETCURSORS)`.

### 3.3 Shake and hold

```
NORMAL ──(shake detected)──► BIG ──(no shake for HoldMillis)──► NORMAL
```

- **Shake:** enough per-axis direction reversals within a sliding window, above
  a noise floor. Sensitivity tunes the thresholds.
- **Hold:** entering BIG arms a timer; each new shake re-arms it. After
  `HoldMillis` (~1 s) with no further shake it returns to NORMAL.
  **Plain movement does not re-arm the timer** — an earlier "shrink when the
  mouse stops moving" rule kept the cursor enlarged during ordinary use and was
  replaced after smoke testing.

### 3.4 Crispness: high-resolution artwork, scaled down

Windows only ships its cursors at 32 px, so *upscaling* them is inherently
blurry. `StyleCrisp` instead embeds high-resolution artwork (the pointer and the
hand) and **scales it down** to the requested size, which stays sharp.

`StyleSystem` keeps the user's real cursors, upscaled and therefore soft. It is
offered because some people prefer their exact cursors over ours.

## 4. Architecture

Hexagonal. The shake-detection domain is pure and fully unit-tested; Windows
sits behind ports with Windows-only adapters.

```
cmd/giant-cursor/     entry point: flags, config, tray wiring, poll loop
cmd/genicon/          assets/icon-source.png       -> .ico (app/tray/installer)
cmd/genarrow/         assets/cursor-raw/*.png      -> embedded cursor artwork

internal/shake/       PURE domain: shake + hold detection (TDD)
internal/config/      sensitivity presets -> shake.Config
internal/cursor/      Enlarger port + Win32 adapter + artwork
internal/input/       PositionSource port + GetCursorPos adapter
internal/app/         state machine: detector -> enlarger side effects
internal/control/     thread-safe live settings (tray writes, poll loop reads)
internal/lifecycle/   tray icon + menu, single instance, cleanup
internal/i18n/        English/Spanish tray labels
```

### Ports

```go
type Enlarger interface {         // internal/cursor
    Enlarge() error
    Restore() error
}

type PositionSource interface {   // internal/input
    Poll() (shake.Point, error)
}
```

`control.Controller` owns the current settings behind a mutex. The poll loop
calls `Step`; the tray calls `SetScale` / `SetSensitivity` / `SetHold` /
`SetStyle`, which normalize the cursor, rebuild the detector and enlarger from a
factory, and persist via a callback. That is why a menu change applies live.

### Dependencies

`golang.org/x/sys` (Win32) and `golang.org/x/image` (downscaling). Standard
library otherwise. `CGO_ENABLED=0`.

## 5. Cursor artwork pipeline

Source art lives in `assets/cursor-raw/*.png`: white shapes with a thick black
outline on a **blue** background. `cmd/genarrow`:

1. Keys out the background — the artwork is neutral (white/black) while the
   backdrop is saturated blue, so "blueness" (B minus the strongest of R/G)
   separates them, with a soft ramp for anti-aliased edges.
2. Crops to the content and writes `internal/cursor/<name>.png`, which is
   embedded with `go:embed`.

At runtime `internal/cursor.artwork` decodes the PNG once, scales it with
CatmullRom to `artFill` (62 %) of the cursor box — matching the visual size of
the upscaled system cursor — and detects the **hotspot** as the middle of the
topmost opaque run (the arrow's apex, the hand's fingertip).

Adding a cursor is data, not code: drop the art in `assets/cursor-raw/`, add the
name to `cursors` in `cmd/genarrow`, and add one entry to `crispArt` in
`internal/cursor/cursor_windows.go`.

## 6. Safety: never leave the cursor stuck

`SetSystemCursor` is global and is **not** reverted when the process dies.
Mitigations, all mandatory:

1. **Startup reset** — every launch restores before doing anything, so a crash
   that left the cursor enlarged is fixed by simply launching again.
2. **Clean exit** — the tray's *Quit* restores and exits.
3. **Shutdown hooks** — `SetConsoleCtrlHandler` restores where the OS allows.
4. **Panic button** — `giant-cursor.exe --restore`.
5. **Uninstall** — the installer runs `--restore`.
6. **Single instance** — a named mutex prevents stacked instances.

The user's normal cursor base size is captured on first run and persisted in
`config.json`, so restore always has a known-good target.

## 7. Configuration

Everything is live-adjustable from the tray and persisted to
`%LOCALAPPDATA%\giant-cursor\config.json`.

| Setting | Default | Notes |
|---|---|---|
| Style | `crisp` | `crisp` or `system` |
| Scale | `4` | 2, 3, 4, 5, 6, 8 |
| Sensitivity | `medium` | `low` / `medium` / `high` |
| Hold | `1000` ms | 700 / 1000 / 1500 |
| Language | auto | `en` / `es`, detected via `GetUserDefaultUILanguage` |
| Autostart | off | `Run` registry key |

Command-line flags mirror these and override the config file for that run.

## 8. Distribution

- **Portable:** the standalone `.exe`, zero dependencies.
- **Installer:** Inno Setup, per-user (no admin), bilingual, autostart checkbox,
  passes the chosen language to the app, restores the cursor on uninstall.
- **Icon:** `assets/icon-source.png` → `cmd/genicon` → multi-resolution `.ico`,
  embedded in the exe (`rsrc_windows.syso`) and loaded by the tray via
  `CreateIconFromResourceEx`.
- **CI:** on a `v*` tag, GitHub Actions tests, builds the exe and the installer,
  and publishes the release.

## 9. Rejected approaches

Recording these so nobody re-litigates them.

| Approach | Why rejected |
|---|---|
| **Native cursor size** (`CursorBaseSize` + `SPI_SETCURSORS`) | The documented way to get crisp large cursors, and the only one that would cover *all* cursor shapes. The registry value was written correctly (verified by read-back) but `SPI_SETCURSORS` returned 0 and the size never applied live — with `uiParam=0`, with the size as `uiParam`, and with a `WM_SETTINGCHANGE` broadcast. Dead on the target machine. The code survives only in `StyleNative`, used by `--restore` because it resets the base size *and* reloads the scheme. |
| **Drawing the pointer from a polygon** | Hand-tuned vertices never matched the reference art; several rounds of "it looks strange". Replaced by scaling down real artwork, which is exact and needs no code to restyle. |
| **Global hotkey to quit** (`Ctrl+Alt+Q`) | On Spanish/Latin-American layouts `Ctrl+Alt` is AltGr, so the hotkey collided with `@`. Replaced by the tray menu. **Never use `Ctrl+Alt`+letter for global hotkeys.** |
| **Synthesized I-beam** | A text caret is essentially thin black lines; the white-fill-with-thick-outline style that suits the pointer and hand turns it into a blocky letter "I". Dropped — the text cursor uses the system zoom. |

## 10. Known limitations

- Only standard system cursors are affected; app-drawn cursors are not.
- In `crisp` mode only the pointer and the hand use artwork; the rest are
  upscaled and softer.
- `system` mode is soft everywhere, because Windows only ships 32px cursors.
