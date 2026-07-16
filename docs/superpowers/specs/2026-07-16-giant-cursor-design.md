# Giant Cursor — Design Document

**Date:** 2026-07-16
**Status:** Approved (pending written-spec review)
**Platform:** Windows (10/11), amd64
**Language:** Go 1.26.2, no CGo (single self-contained `.exe`)

---

## 1. Problem

People with low vision routinely lose track of the mouse cursor on screen. macOS
solves this with a "shake to locate" gesture: shaking the mouse temporarily
magnifies the pointer so it becomes impossible to miss, then it shrinks back.

Windows has no built-in equivalent. PowerToys "Find My Mouse" approximates it but
is unsatisfactory for two reasons:

1. **Overhead** — it is a .NET component and draws a full-screen overlay
   (spotlight), which forces whole-screen composition every frame.
2. **Unreliability under load** — it relies on a low-level global mouse hook
   (`WH_MOUSE_LL`). When the system is busy with many applications, Windows can
   starve or silently detach the hook (callback timeout), so the effect
   intermittently stops working.

## 2. Goal

A tiny, fast, reliable Windows utility that reproduces the macOS "shake to
enlarge the cursor" behavior, built to be publicly distributed as an accessibility
tool for everyone.

### Success criteria

- Shaking the mouse enlarges the **actual system cursor** (default 4x).
- The cursor stays enlarged while moving and returns to normal after ~1s idle.
- Negligible CPU/memory footprint; works reliably even with many apps open.
- Never leaves the cursor stuck enlarged, even after a crash or forced kill.
- Easy to install (portable `.exe` + a friendly installer) and well documented.

### Non-goals (YAGNI)

- No spotlight/halo effect (explicitly rejected).
- No settings GUI window.
- No system-tray icon.
- No support for fully custom (non-standard) application cursors in v1.
- No pixel-perfect crispness at 4x in v1 (documented tradeoff; see §10).

## 3. Core technical decisions

### 3.1 Detection by polling, NOT by hook

A dedicated goroutine polls `GetCursorPos` at ~120 Hz (every ~8 ms). This is the
key decision that fixes PowerToys' reliability problem: polling cannot be starved
or detached the way a low-level hook can. CPU cost is negligible.

### 3.2 Enlargement via `SetSystemCursor`

On shake, the standard system cursors are swapped for pre-scaled larger copies via
`SetSystemCursor`. There is **no overlay window, no per-frame drawing, and no
screen composition** — the OS renders the (bigger) cursor exactly as it always
does. This is the lightest possible mechanism.

Restore is done by reloading the user's configured cursor scheme with
`SystemParametersInfo(SPI_SETCURSORS)`, rather than manually tracking and
restoring individual handles.

### 3.3 State machine

```
NORMAL ──(shake detected)──► BIG ──(idle ~1s)──► NORMAL
```

- **Shake detection:** keep a short ring buffer of recent (position, timestamp)
  samples; count direction reversals on the X and Y axes within a sliding window
  (~400 ms) above a minimum speed threshold. Crossing the reversal threshold
  triggers the transition to BIG. Sensitivity is configurable.
- **Idle detection:** while BIG, if the cursor does not move beyond a small
  threshold for ~1 s, transition back to NORMAL.

## 4. Architecture

Hexagonal / "screaming" architecture. The shake-detection domain is **pure** (no
Windows dependency), so it is fully unit-testable without Windows. Windows sits
behind ports with Windows-only adapters.

```
giant-cursor/
├─ cmd/giant-cursor/
│  └─ main.go                     # flags, lifecycle wiring, message loop
├─ internal/shake/
│  ├─ detector.go                 # PURE domain: shake + idle detection
│  └─ detector_test.go            # golden cases (TDD)
├─ internal/cursor/
│  ├─ cursor.go                   # Enlarger port (interface)
│  └─ cursor_windows.go           # Win32 adapter (SetSystemCursor / SPI_SETCURSORS)
├─ internal/input/
│  ├─ source.go                   # PositionSource port (interface)
│  └─ poller_windows.go           # GetCursorPos polling adapter
├─ internal/app/
│  └─ app.go                      # state machine: wires detector + input + cursor
├─ internal/lifecycle/
│  ├─ hotkey_windows.go           # RegisterHotKey (Ctrl+Alt+Q) + message loop
│  ├─ instance_windows.go         # single-instance named mutex
│  └─ cleanup_windows.go          # console/session shutdown → restore cursors
├─ internal/config/
│  └─ config.go                   # flags + config.json load/save
├─ installer/
│  └─ giant-cursor.iss            # Inno Setup script
├─ .github/workflows/release.yml  # build portable exe + installer on tag
├─ go.mod
└─ README.md                      # English, public
```

### Ports

```go
// internal/input/source.go
type Point struct{ X, Y int32 }

type PositionSource interface {
    // Poll returns the current cursor position.
    Poll() (Point, error)
}

// internal/cursor/cursor.go
type Enlarger interface {
    Enlarge() error   // swap standard cursors to scaled copies
    Restore() error   // reload user's cursor scheme (SPI_SETCURSORS)
}
```

The domain (`shake.Detector`) consumes samples and emits state transitions; it
never imports `cursor` or `input` adapters. `app.App` wires them together.

### Dependencies

- `golang.org/x/sys/windows` (v0.47.0) — Win32 syscalls.
- Standard library only otherwise. `CGO_ENABLED=0`.

## 5. Windows integration details

### Cursors covered (standard `OCR_*` ids)

`OCR_NORMAL`, `OCR_IBEAM`, `OCR_WAIT`, `OCR_CROSS`, `OCR_UP`, `OCR_SIZENWSE`,
`OCR_SIZENESW`, `OCR_SIZEWE`, `OCR_SIZENS`, `OCR_SIZEALL`, `OCR_NO`, `OCR_HAND`,
`OCR_APPSTARTING`, `OCR_HELP`.

For each id: load the current cursor and build a scaled copy at `base * scale`
using `LoadImageW` / `CopyImage`. Because `SetSystemCursor` takes ownership of the
handle it is given (and destroys it), a **fresh** scaled copy is created for each
`SetSystemCursor` call.

### Enlarge

For each covered id: `SetSystemCursor(freshBigCopy, id)`.

### Restore

`SystemParametersInfo(SPI_SETCURSORS, 0, nil, 0)` reloads the user's configured
scheme — clean and does not depend on us holding original handles.

## 6. Safety: never leave the cursor stuck enlarged

`SetSystemCursor` changes are **global** and are **not** reverted when the process
dies. Mitigations (all mandatory):

1. **Startup reset:** on launch, call `SPI_SETCURSORS` first to establish a clean
   baseline. If a previous run crashed while BIG, simply relaunching fixes it.
2. **Clean-restore path:** restore always uses `SPI_SETCURSORS`.
3. **Clean exit via hotkey:** `Ctrl+Alt+Q` restores cursors and exits.
4. **Extra shutdown hooks:** `SetConsoleCtrlHandler` and
   `WM_QUERYENDSESSION`/`WM_ENDSESSION` restore where the OS gives us a chance.
5. **Panic button:** `giant-cursor.exe --restore` restores cursors and exits.
6. **Single instance:** a named mutex prevents stacked instances.

Note: a hard `TerminateProcess` (Task Manager "End Task") cannot run our code —
mitigation #1 (startup reset) and #5 (`--restore`) cover that case.

## 7. Configuration

Command-line flags with sensible defaults:

| Flag | Default | Meaning |
|---|---|---|
| `--scale` | `4` | Enlargement factor. |
| `--sensitivity` | `medium` | Shake sensitivity (`low`/`medium`/`high`). |
| `--idle-ms` | `1000` | Idle time before shrinking back. |
| `--hotkey` | `ctrl+alt+q` | Clean-exit hotkey. |

Lifecycle flags:

| Flag | Action |
|---|---|
| `--install` | Register autostart + save `config.json`, then start. |
| `--uninstall` | Remove autostart, restore cursors. |
| `--restore` | Restore cursors and exit (panic button). |

`--install` persists chosen settings to
`%LOCALAPPDATA%\giant-cursor\config.json`, read on startup so autostart carries
the user's configuration. Running with no window is the default; there is no tray
icon and no settings window.

## 8. Distribution

- **Portable:** the standalone `.exe` — download and run, zero dependencies.
- **Installer:** an **Inno Setup** package (`GiantCursorSetup.exe`) with a familiar
  wizard, a Start Menu shortcut, and a "Start with Windows" checkbox. Inno Setup is
  a build-time dependency only (not required by end users).
- **Autostart:** via the `Run` registry key, toggled by the installer checkbox or
  `--install` / `--uninstall`.
- **CI:** a GitHub Actions workflow builds the portable `.exe` and the installer on
  tagged releases.
- **Future:** winget / scoop manifests.

## 9. Testing (Strict TDD)

The `shake` package is pure logic and is developed test-first with synthetic
sequences of `(position, timestamp)` samples. Golden cases:

- A vigorous back-and-forth shake **triggers** BIG.
- Normal straight movement does **not** trigger.
- Slow drift does **not** trigger.
- Diagonal shake **triggers**.
- Sensitivity thresholds behave monotonically (higher sensitivity → easier trigger).
- After BIG, an idle gap ≥ `idle-ms` returns to NORMAL; continued movement keeps BIG.

The Win32 adapters (`cursor_windows.go`, `input/poller_windows.go`) are thin and
verified manually on Windows. Ports allow the domain tests to run with fake
adapters on any OS.

## 10. Known tradeoff (accepted)

At 4x, scaling a 32px standard cursor to 128px via `CopyImage` looks **somewhat
pixelated**. This is accepted for v1: the priority is never losing the cursor, and
the runtime-scaling approach is simple and light. A future v1.1 may bundle
high-resolution large cursor assets for pixel-perfect crispness.

## 11. Open items for implementation planning

- Exact reversal-count and speed thresholds per sensitivity level (tuned during
  TDD, then validated manually).
- Whether to also cover `OCR_SIZE`/legacy aliases.
- Message-loop threading model (`runtime.LockOSThread` for the loop goroutine).
