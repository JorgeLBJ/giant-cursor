# Giant Cursor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a tiny, fast Windows utility in Go that enlarges the system cursor (4x) when the user shakes the mouse, then shrinks it back after ~1s idle — a macOS-style "shake to locate" for low-vision accessibility.

**Architecture:** Hexagonal. A pure `shake` domain (shake + idle detection) is developed test-first and is fully unit-testable without Windows. Windows sits behind two ports — `input.PositionSource` (polling `GetCursorPos`) and `cursor.Enlarger` (`SetSystemCursor` / `SPI_SETCURSORS`). An `app.App` state machine wires them: on the NORMAL→BIG transition it enlarges, on BIG→NORMAL it restores.

**Tech Stack:** Go 1.26.2, `golang.org/x/sys/windows` v0.47.0, `CGO_ENABLED=0`, single self-contained `.exe`. Inno Setup for the installer. GitHub Actions for releases.

## Global Constraints

- Platform: Windows 10/11, `amd64`. Build with `CGO_ENABLED=0`.
- Go version floor: `1.26` (uses builtin `max`; developed on `1.26.2`).
- Only external dependency: `golang.org/x/sys` `v0.47.0`. Standard library otherwise.
- Go module path: `giant-cursor` (local). Can be renamed to a GitHub path before publishing via search-replace of `giant-cursor/internal`.
- All artifacts (code, comments, docs, README) in **English**. README is public-facing.
- Commits: Conventional Commits. **Never** add `Co-Authored-By` or AI attribution.
- Default enlargement scale: **4**. Default clean-exit hotkey: **Ctrl+Alt+Q**.
- Windows-specific adapter files use the `//go:build windows` tag so the pure domain still tests on any OS.

---

## File Structure

```
giant-cursor/
├─ go.mod
├─ cmd/giant-cursor/main.go        # flags, config, lifecycle wiring, run loop, hotkey loop
├─ internal/shake/
│  ├─ detector.go                  # PURE: Point, Sample, State, Config, Detector
│  └─ detector_test.go             # golden TDD cases
├─ internal/config/
│  ├─ config.go                    # sensitivity presets -> shake.Config
│  └─ config_test.go
├─ internal/cursor/
│  ├─ cursor.go                    # Enlarger port (interface)
│  └─ cursor_windows.go            # Win32 adapter (SetSystemCursor / SPI_SETCURSORS)
├─ internal/input/
│  ├─ source.go                    # PositionSource port (interface)
│  └─ poller_windows.go            # GetCursorPos adapter
├─ internal/app/
│  ├─ app.go                       # state machine: detector + enlarger
│  └─ app_test.go                  # transition tests with a fake enlarger
├─ internal/lifecycle/
│  ├─ instance_windows.go          # single-instance named mutex
│  ├─ hotkey_windows.go            # RegisterHotKey + message loop
│  └─ cleanup_windows.go           # SetConsoleCtrlHandler
├─ installer/giant-cursor.iss      # Inno Setup script
├─ .github/workflows/release.yml   # build exe + installer on tag
├─ LICENSE
└─ README.md
```

**Task dependency order:** 1 → 2 → 3 (pure domain, TDD) → 4 (config, TDD) → 5 (ports + app, TDD) → 6, 7 (Win32 adapters, manual verify) → 8 (lifecycle, manual verify) → 9 (main wiring, end-to-end verify) → 10 (installer) → 11 (README + CI).

Tasks 1–5 are pure Go, test-first, and run on any OS. Tasks 6–9 are Win32 syscall glue: they cannot be meaningfully unit-tested, so each ends with a **manual verification on Windows** (build + run + observe) instead of an automated assertion.

---

### Task 1: Module scaffold + `shake` domain skeleton

**Files:**
- Create: `go.mod`
- Create: `internal/shake/detector.go`
- Test: `internal/shake/detector_test.go`

**Interfaces:**
- Produces: `shake.Point{X, Y int32}`, `shake.Sample{Pos Point; Millis int64}`, `shake.State` (`StateNormal`, `StateBig`), `shake.Config`, `shake.New(Config) *Detector`, `(*Detector).State() State`, `(*Detector).Update(Sample) State`.

- [ ] **Step 1: Create the module**

Run:
```bash
cd "F:/PROYECTOS_NODE/giant-cursor"
go mod init giant-cursor
```
Expected: creates `go.mod` with `module giant-cursor` and `go 1.26`.

- [ ] **Step 2: Write the failing test**

Create `internal/shake/detector_test.go`:
```go
package shake

import "testing"

func TestNewDetectorStartsNormal(t *testing.T) {
	d := New(Config{})
	if d.State() != StateNormal {
		t.Fatalf("want NORMAL, got %v", d.State())
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/shake/`
Expected: FAIL — build error, `undefined: New` / `undefined: Config` / `undefined: StateNormal`.

- [ ] **Step 4: Write minimal implementation**

Create `internal/shake/detector.go`:
```go
// Package shake implements pure, OS-independent mouse-shake and idle
// detection. It consumes cursor position samples and reports whether the
// cursor should currently be enlarged (BIG) or normal.
package shake

// Point is a cursor position in screen pixels.
type Point struct{ X, Y int32 }

// Sample is a cursor position observed at a monotonic timestamp (ms).
type Sample struct {
	Pos    Point
	Millis int64
}

// State is the desired cursor state.
type State int

const (
	StateNormal State = iota
	StateBig
)

func (s State) String() string {
	if s == StateBig {
		return "BIG"
	}
	return "NORMAL"
}

// Config tunes shake and idle detection.
type Config struct {
	WindowMillis   int64 // sliding window for counting reversals
	MinReversals   int   // reversals within window to trigger BIG
	NoiseFloor     int32 // per-step axis delta below this (px) is ignored
	IdleMillis     int64 // idle time before returning to NORMAL
	IdleMoveThresh int32 // per-step move below this (px) counts as idle
}

// Detector is a stateful shake/idle detector. Not safe for concurrent use;
// call Update from a single goroutine.
type Detector struct {
	cfg            Config
	samples        []Sample
	state          State
	lastMoveMillis int64
	lastPos        Point
	haveLast       bool
}

// New returns a Detector in the NORMAL state.
func New(cfg Config) *Detector {
	return &Detector{cfg: cfg, state: StateNormal}
}

// State returns the current state.
func (d *Detector) State() State { return d.state }

// Update feeds one sample and returns the resulting state.
func (d *Detector) Update(s Sample) State {
	// Placeholder until Task 2/3 implement the algorithm.
	return d.state
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/shake/`
Expected: PASS (`ok  giant-cursor/internal/shake`).

- [ ] **Step 6: Commit**

```bash
git add go.mod internal/shake/
git commit -m "feat: scaffold module and shake domain skeleton"
```

---

### Task 2: Shake detection — reversal counting triggers BIG

**Files:**
- Modify: `internal/shake/detector.go`
- Test: `internal/shake/detector_test.go`

**Interfaces:**
- Consumes: everything from Task 1.
- Produces: real `Update` behaviour for the NORMAL→BIG transition; unexported `reversals()` and `axisReversals(xAxis bool) int`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/shake/detector_test.go`:
```go
// feed drives a sequence of positions at a fixed time step and returns the
// final state.
func feed(d *Detector, xs, ys []int32, stepMillis int64) State {
	var st State
	var ms int64
	for i := range xs {
		st = d.Update(Sample{Pos: Point{X: xs[i], Y: ys[i]}, Millis: ms})
		ms += stepMillis
	}
	return st
}

func shakeCfg() Config {
	return Config{WindowMillis: 500, MinReversals: 4, NoiseFloor: 3, IdleMillis: 1000, IdleMoveThresh: 3}
}

func TestHorizontalShakeTriggersBig(t *testing.T) {
	d := New(shakeCfg())
	xs := []int32{0, 30, 0, 30, 0, 30, 0}
	ys := []int32{0, 0, 0, 0, 0, 0, 0}
	if got := feed(d, xs, ys, 20); got != StateBig {
		t.Fatalf("want BIG after shake, got %v", got)
	}
}

func TestStraightMovementStaysNormal(t *testing.T) {
	d := New(shakeCfg())
	xs := []int32{0, 20, 40, 60, 80, 100}
	ys := []int32{0, 20, 40, 60, 80, 100}
	if got := feed(d, xs, ys, 20); got != StateNormal {
		t.Fatalf("want NORMAL for straight move, got %v", got)
	}
}

func TestSlowDriftBelowNoiseFloorStaysNormal(t *testing.T) {
	d := New(shakeCfg()) // NoiseFloor = 3
	xs := []int32{0, 1, 0, 2, 1, 2, 0} // every delta abs < 3 -> ignored
	ys := []int32{0, 0, 0, 0, 0, 0, 0}
	if got := feed(d, xs, ys, 20); got != StateNormal {
		t.Fatalf("want NORMAL for sub-noise drift, got %v", got)
	}
}

func TestDiagonalShakeTriggersBig(t *testing.T) {
	d := New(shakeCfg())
	xs := []int32{0, 30, 0, 30, 0, 30, 0}
	ys := []int32{0, 30, 0, 30, 0, 30, 0}
	if got := feed(d, xs, ys, 20); got != StateBig {
		t.Fatalf("want BIG for diagonal shake, got %v", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/shake/ -run 'Shake|Movement|Drift'`
Expected: FAIL — `TestHorizontalShakeTriggersBig` and `TestDiagonalShakeTriggersBig` get NORMAL (the placeholder never transitions).

- [ ] **Step 3: Write the implementation**

Replace the body of `Update` and add the helpers in `internal/shake/detector.go`. Replace the placeholder `Update` method with:
```go
func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// Update feeds one sample and returns the resulting state.
func (d *Detector) Update(s Sample) State {
	if d.haveLast {
		moved := abs32(s.Pos.X - d.lastPos.X)
		if dy := abs32(s.Pos.Y - d.lastPos.Y); dy > moved {
			moved = dy
		}
		if moved > d.cfg.IdleMoveThresh {
			d.lastMoveMillis = s.Millis
		}
	} else {
		d.lastMoveMillis = s.Millis
	}
	d.haveLast = true
	d.lastPos = s.Pos

	d.samples = append(d.samples, s)
	cutoff := s.Millis - d.cfg.WindowMillis
	drop := 0
	for drop < len(d.samples) && d.samples[drop].Millis < cutoff {
		drop++
	}
	if drop > 0 {
		d.samples = d.samples[drop:]
	}

	if d.state == StateNormal && d.reversals() >= d.cfg.MinReversals {
		d.state = StateBig
		d.samples = d.samples[:0] // require a fresh shake next time
	}
	return d.state
}

// reversals returns the larger per-axis direction-reversal count in the
// current window, so a pure horizontal or vertical shake counts as well as a
// diagonal one.
func (d *Detector) reversals() int {
	return max(d.axisReversals(true), d.axisReversals(false))
}

// axisReversals counts sign changes of the per-step delta along one axis,
// ignoring steps whose delta magnitude is below NoiseFloor.
func (d *Detector) axisReversals(xAxis bool) int {
	var lastSign int
	count := 0
	var prev Sample
	have := false
	for _, s := range d.samples {
		if !have {
			prev, have = s, true
			continue
		}
		var delta int32
		if xAxis {
			delta = s.Pos.X - prev.Pos.X
		} else {
			delta = s.Pos.Y - prev.Pos.Y
		}
		prev = s
		if abs32(delta) < d.cfg.NoiseFloor {
			continue
		}
		sign := 1
		if delta < 0 {
			sign = -1
		}
		if lastSign != 0 && sign != lastSign {
			count++
		}
		lastSign = sign
	}
	return count
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/shake/`
Expected: PASS — all four new tests plus Task 1's test.

- [ ] **Step 5: Commit**

```bash
git add internal/shake/
git commit -m "feat: detect mouse shake via windowed direction reversals"
```

---

### Task 3: Idle detection — return to NORMAL after idle

**Files:**
- Modify: `internal/shake/detector.go`
- Test: `internal/shake/detector_test.go`

**Interfaces:**
- Consumes: Task 2 detector.
- Produces: the BIG→NORMAL transition inside `Update` (no new exported symbols).

- [ ] **Step 1: Write the failing tests**

Append to `internal/shake/detector_test.go`:
```go
func TestIdleReturnsToNormal(t *testing.T) {
	d := New(shakeCfg())
	feed(d, []int32{0, 30, 0, 30, 0, 30, 0}, []int32{0, 0, 0, 0, 0, 0, 0}, 20)
	if d.State() != StateBig {
		t.Fatalf("precondition failed: want BIG, got %v", d.State())
	}
	// Hold still well past IdleMillis (1000ms): same position, later timestamp.
	st := d.Update(Sample{Pos: d.lastPos, Millis: 10000})
	if st != StateNormal {
		t.Fatalf("want NORMAL after idle, got %v", st)
	}
}

func TestContinuedMovementKeepsBig(t *testing.T) {
	d := New(shakeCfg())
	feed(d, []int32{0, 30, 0, 30, 0, 30, 0}, []int32{0, 0, 0, 0, 0, 0, 0}, 20)
	if d.State() != StateBig {
		t.Fatalf("precondition failed: want BIG, got %v", d.State())
	}
	// Keep moving > IdleMoveThresh with time steps < IdleMillis.
	var ms int64 = 200
	var x int32
	st := d.State()
	for i := 0; i < 20; i++ {
		x += 10
		ms += 50
		st = d.Update(Sample{Pos: Point{X: x}, Millis: ms})
	}
	if st != StateBig {
		t.Fatalf("want BIG while still moving, got %v", st)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/shake/ -run 'Idle|KeepsBig'`
Expected: FAIL — `TestIdleReturnsToNormal` gets BIG (no idle transition yet).

- [ ] **Step 3: Write the implementation**

In `internal/shake/detector.go`, extend the state logic in `Update`. Replace:
```go
	if d.state == StateNormal && d.reversals() >= d.cfg.MinReversals {
		d.state = StateBig
		d.samples = d.samples[:0] // require a fresh shake next time
	}
	return d.state
```
with:
```go
	switch d.state {
	case StateNormal:
		if d.reversals() >= d.cfg.MinReversals {
			d.state = StateBig
			d.samples = d.samples[:0] // require a fresh shake next time
		}
	case StateBig:
		if s.Millis-d.lastMoveMillis >= d.cfg.IdleMillis {
			d.state = StateNormal
			d.samples = d.samples[:0]
		}
	}
	return d.state
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/shake/`
Expected: PASS — full `shake` suite green.

- [ ] **Step 5: Commit**

```bash
git add internal/shake/
git commit -m "feat: shrink cursor back to normal after idle"
```

---

### Task 4: `config` package — sensitivity presets

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consumes: `shake.Config` from Task 1.
- Produces: `config.Sensitivity` (`Low`, `Medium`, `High`), `config.ShakeConfig(s Sensitivity, idleMillis int64) shake.Config`.

- [ ] **Step 1: Write the failing tests**

Create `internal/config/config_test.go`:
```go
package config

import "testing"

func TestSensitivityOrdering(t *testing.T) {
	if ShakeConfig(High, 1000).MinReversals >= ShakeConfig(Low, 1000).MinReversals {
		t.Fatal("High sensitivity must need fewer reversals than Low")
	}
}

func TestUnknownDefaultsToMedium(t *testing.T) {
	if ShakeConfig("bogus", 1000).MinReversals != ShakeConfig(Medium, 1000).MinReversals {
		t.Fatal("unknown sensitivity should default to Medium")
	}
}

func TestIdleMillisPassthrough(t *testing.T) {
	if ShakeConfig(Medium, 750).IdleMillis != 750 {
		t.Fatal("idle millis not propagated")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/`
Expected: FAIL — `undefined: ShakeConfig` / `undefined: High`.

- [ ] **Step 3: Write the implementation**

Create `internal/config/config.go`:
```go
// Package config maps human-friendly sensitivity levels to a shake.Config.
package config

import "giant-cursor/internal/shake"

// Sensitivity names how vigorous a shake must be to trigger enlargement.
type Sensitivity string

const (
	Low    Sensitivity = "low"
	Medium Sensitivity = "medium"
	High   Sensitivity = "high"
)

// ShakeConfig builds a shake.Config for the given sensitivity and idle timeout.
// Unknown values fall back to Medium. Starting thresholds; tune with real use.
func ShakeConfig(s Sensitivity, idleMillis int64) shake.Config {
	cfg := shake.Config{
		WindowMillis:   400,
		MinReversals:   4,
		NoiseFloor:     4,
		IdleMillis:     idleMillis,
		IdleMoveThresh: 3,
	}
	switch s {
	case High:
		cfg.MinReversals = 3
		cfg.WindowMillis = 500
		cfg.NoiseFloor = 2
	case Low:
		cfg.MinReversals = 6
		cfg.WindowMillis = 350
		cfg.NoiseFloor = 6
	default: // Medium and any unknown value
	}
	return cfg
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: map sensitivity levels to shake config"
```

---

### Task 5: Ports + `app` state machine

**Files:**
- Create: `internal/cursor/cursor.go`
- Create: `internal/input/source.go`
- Create: `internal/app/app.go`
- Test: `internal/app/app_test.go`

**Interfaces:**
- Consumes: `shake.Detector`, `shake.Sample`, `shake.State`.
- Produces:
  - `cursor.Enlarger` interface: `Enlarge() error`, `Restore() error`.
  - `input.PositionSource` interface: `Poll() (shake.Point, error)`.
  - `app.New(det *shake.Detector, cur cursor.Enlarger) *App`, `(*App).Step(shake.Sample) error`.

- [ ] **Step 1: Write the port interfaces**

Create `internal/cursor/cursor.go`:
```go
// Package cursor defines the port for enlarging and restoring the OS cursor.
package cursor

// Enlarger swaps the system cursors to enlarged copies and restores them.
type Enlarger interface {
	Enlarge() error
	Restore() error
}
```

Create `internal/input/source.go`:
```go
// Package input defines the port for sampling the cursor position.
package input

import "giant-cursor/internal/shake"

// PositionSource returns the current cursor position.
type PositionSource interface {
	Poll() (shake.Point, error)
}
```

- [ ] **Step 2: Write the failing test**

Create `internal/app/app_test.go`:
```go
package app

import (
	"testing"

	"giant-cursor/internal/shake"
)

type fakeEnlarger struct{ enlarged, restored int }

func (f *fakeEnlarger) Enlarge() error { f.enlarged++; return nil }
func (f *fakeEnlarger) Restore() error { f.restored++; return nil }

func drive(a *App, xs []int32, stepMillis, startMillis int64) {
	ms := startMillis
	for _, x := range xs {
		_ = a.Step(shake.Sample{Pos: shake.Point{X: x}, Millis: ms})
		ms += stepMillis
	}
}

func TestEnlargeOnShakeRestoreOnIdle(t *testing.T) {
	fe := &fakeEnlarger{}
	det := shake.New(shake.Config{
		WindowMillis: 500, MinReversals: 4, NoiseFloor: 3,
		IdleMillis: 1000, IdleMoveThresh: 3,
	})
	a := New(det, fe)

	drive(a, []int32{0, 30, 0, 30, 0, 30, 0}, 20, 0)
	if fe.enlarged != 1 {
		t.Fatalf("want 1 Enlarge, got %d", fe.enlarged)
	}
	if fe.restored != 0 {
		t.Fatalf("want 0 Restore before idle, got %d", fe.restored)
	}

	// Idle: same position far in the future -> BIG->NORMAL -> one Restore.
	_ = a.Step(shake.Sample{Pos: shake.Point{X: 0}, Millis: 10000})
	if fe.restored != 1 {
		t.Fatalf("want 1 Restore after idle, got %d", fe.restored)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/app/`
Expected: FAIL — `undefined: New` / `undefined: App`.

- [ ] **Step 4: Write the implementation**

Create `internal/app/app.go`:
```go
// Package app wires the shake detector to a cursor Enlarger, calling Enlarge
// on the NORMAL->BIG transition and Restore on BIG->NORMAL.
package app

import (
	"giant-cursor/internal/cursor"
	"giant-cursor/internal/shake"
)

// App drives the enlarge/restore side effects from detector state changes.
type App struct {
	det   *shake.Detector
	cur   cursor.Enlarger
	state shake.State
}

// New returns an App in the NORMAL state.
func New(det *shake.Detector, cur cursor.Enlarger) *App {
	return &App{det: det, cur: cur, state: shake.StateNormal}
}

// Step feeds one sample to the detector and applies the resulting side effect.
func (a *App) Step(s shake.Sample) error {
	prev := a.state
	next := a.det.Update(s)
	a.state = next
	switch {
	case prev == shake.StateNormal && next == shake.StateBig:
		return a.cur.Enlarge()
	case prev == shake.StateBig && next == shake.StateNormal:
		return a.cur.Restore()
	}
	return nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./...`
Expected: PASS across `shake`, `config`, and `app` (the Windows adapters do not exist yet, so nothing else is compiled on non-Windows).

- [ ] **Step 6: Commit**

```bash
git add internal/cursor/cursor.go internal/input/source.go internal/app/
git commit -m "feat: app state machine wiring detector to cursor port"
```

---

### Task 6: Win32 cursor adapter

**Files:**
- Create: `internal/cursor/cursor_windows.go`

**Interfaces:**
- Consumes: implements `cursor.Enlarger`.
- Produces: `cursor.NewWin32(scale int) *Win32` with `Enlarge()`/`Restore()`.

This task is Win32 syscall glue — no unit test. It ends with a manual build check.

- [ ] **Step 1: Add the dependency**

Run:
```bash
go get golang.org/x/sys/windows@v0.47.0
```
Expected: `go.mod` now requires `golang.org/x/sys v0.47.0`.

- [ ] **Step 2: Write the adapter**

Create `internal/cursor/cursor_windows.go`:
```go
//go:build windows

package cursor

import (
	"fmt"

	"golang.org/x/sys/windows"
)

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	procLoadImageW      = user32.NewProc("LoadImageW")
	procCopyImage       = user32.NewProc("CopyImage")
	procSetSystemCursor = user32.NewProc("SetSystemCursor")
	procSystemParamInfo = user32.NewProc("SystemParametersInfoW")
	procGetSystemMetric = user32.NewProc("GetSystemMetrics")
)

const (
	imageCursor    = 2      // IMAGE_CURSOR
	lrDefaultSize  = 0x0040 // LR_DEFAULTSIZE
	lrShared       = 0x8000 // LR_SHARED
	smCXCursor     = 13     // SM_CXCURSOR
	smCYCursor     = 14     // SM_CYCURSOR
	spiSetCursors  = 0x0057 // SPI_SETCURSORS
)

// Standard system cursor ids (OCR_*). Covers the common shapes; any id the
// system does not provide is skipped at runtime.
var systemCursorIDs = []uintptr{
	32512, // OCR_NORMAL
	32513, // OCR_IBEAM
	32514, // OCR_WAIT
	32515, // OCR_CROSS
	32516, // OCR_UP
	32642, // OCR_SIZENWSE
	32643, // OCR_SIZENESW
	32644, // OCR_SIZEWE
	32645, // OCR_SIZENS
	32646, // OCR_SIZEALL
	32648, // OCR_NO
	32649, // OCR_HAND
	32650, // OCR_APPSTARTING
	32651, // OCR_HELP
}

// Win32 enlarges the standard system cursors and restores the user's scheme.
type Win32 struct{ scale int }

// NewWin32 returns a Win32 enlarger with the given scale factor (e.g. 4).
func NewWin32(scale int) *Win32 {
	if scale < 1 {
		scale = 1
	}
	return &Win32{scale: scale}
}

func metric(index uintptr) int {
	r, _, _ := procGetSystemMetric.Call(index)
	if r == 0 {
		return 32
	}
	return int(r)
}

// Enlarge swaps every standard system cursor for a scaled copy. It loads the
// shared system cursor, makes an owned scaled copy with CopyImage, and hands
// that copy to SetSystemCursor (which takes ownership).
func (w *Win32) Enlarge() error {
	cx := uintptr(metric(smCXCursor) * w.scale)
	cy := uintptr(metric(smCYCursor) * w.scale)
	for _, id := range systemCursorIDs {
		hShared, _, _ := procLoadImageW.Call(0, id, imageCursor, 0, 0, lrShared|lrDefaultSize)
		if hShared == 0 {
			continue
		}
		hBig, _, _ := procCopyImage.Call(hShared, imageCursor, cx, cy, 0)
		if hBig == 0 {
			continue
		}
		procSetSystemCursor.Call(hBig, id)
	}
	return nil
}

// Restore reloads the user's configured cursor scheme.
func (w *Win32) Restore() error {
	r, _, err := procSystemParamInfo.Call(spiSetCursors, 0, 0, 0)
	if r == 0 {
		return fmt.Errorf("SPI_SETCURSORS failed: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Verify it builds**

Run: `go build ./...`
Expected: builds with no errors (on Windows).

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/cursor/cursor_windows.go
git commit -m "feat: win32 cursor enlarge/restore adapter"
```

---

### Task 7: Win32 input poller

**Files:**
- Create: `internal/input/poller_windows.go`

**Interfaces:**
- Consumes: implements `input.PositionSource`; returns `shake.Point`.
- Produces: `input.NewWin32Poller() *Win32Poller` with `Poll() (shake.Point, error)`.

- [ ] **Step 1: Write the adapter**

Create `internal/input/poller_windows.go`:
```go
//go:build windows

package input

import (
	"fmt"
	"unsafe"

	"giant-cursor/internal/shake"
	"golang.org/x/sys/windows"
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	procGetCursorPos = user32.NewProc("GetCursorPos")
)

type winPoint struct{ X, Y int32 }

// Win32Poller reads the cursor position via GetCursorPos.
type Win32Poller struct{}

// NewWin32Poller returns a GetCursorPos-based position source.
func NewWin32Poller() *Win32Poller { return &Win32Poller{} }

// Poll returns the current cursor position in screen coordinates.
func (p *Win32Poller) Poll() (shake.Point, error) {
	var pt winPoint
	r, _, err := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if r == 0 {
		return shake.Point{}, fmt.Errorf("GetCursorPos failed: %w", err)
	}
	return shake.Point{X: pt.X, Y: pt.Y}, nil
}
```

- [ ] **Step 2: Verify it builds**

Run: `go build ./...`
Expected: builds with no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/input/poller_windows.go
git commit -m "feat: win32 GetCursorPos polling adapter"
```

---

### Task 8: Lifecycle — single instance, hotkey loop, console cleanup

**Files:**
- Create: `internal/lifecycle/instance_windows.go`
- Create: `internal/lifecycle/hotkey_windows.go`
- Create: `internal/lifecycle/cleanup_windows.go`

**Interfaces:**
- Produces:
  - `lifecycle.AcquireSingleInstance(name string) (release func(), already bool, err error)`
  - `lifecycle.RunHotkeyLoop(onExit func())` — registers Ctrl+Alt+Q, blocks until pressed (must run on an OS-locked thread).
  - `lifecycle.OnConsoleClose(handler func())`

- [ ] **Step 1: Write the single-instance guard**

Create `internal/lifecycle/instance_windows.go`:
```go
//go:build windows

package lifecycle

import "golang.org/x/sys/windows"

// AcquireSingleInstance creates a named mutex. If it already exists, already
// is true and the caller should exit. Otherwise release must be called on exit.
func AcquireSingleInstance(name string) (release func(), already bool, err error) {
	ptr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, false, err
	}
	h, err := windows.CreateMutex(nil, false, ptr)
	if h == 0 {
		return nil, false, err
	}
	if err == windows.ERROR_ALREADY_EXISTS {
		windows.CloseHandle(h)
		return nil, true, nil
	}
	return func() { windows.CloseHandle(h) }, false, nil
}
```

- [ ] **Step 2: Write the hotkey message loop**

Create `internal/lifecycle/hotkey_windows.go`:
```go
//go:build windows

package lifecycle

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32u          = windows.NewLazySystemDLL("user32.dll")
	procRegisterHK   = user32u.NewProc("RegisterHotKey")
	procGetMessageW  = user32u.NewProc("GetMessageW")
	procTranslateMsg = user32u.NewProc("TranslateMessage")
	procDispatchMsgW = user32u.NewProc("DispatchMessageW")
)

const (
	modAlt     = 0x0001 // MOD_ALT
	modControl = 0x0002 // MOD_CONTROL
	wmHotkey   = 0x0312 // WM_HOTKEY
	vkQ        = 0x51   // 'Q'
)

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ X, Y int32 }
}

// RunHotkeyLoop registers Ctrl+Alt+Q and blocks, pumping messages, until the
// hotkey fires (then it calls onExit and returns) or GetMessage fails. Call
// runtime.LockOSThread() on the calling goroutine first: the message queue is
// thread-specific.
func RunHotkeyLoop(onExit func()) {
	procRegisterHK.Call(0, 1, modControl|modAlt, vkQ)
	var m winMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 { // 0 = WM_QUIT, -1 = error
			break
		}
		if m.message == wmHotkey {
			onExit()
			return
		}
		procTranslateMsg.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMsgW.Call(uintptr(unsafe.Pointer(&m)))
	}
}
```

- [ ] **Step 3: Write the console-close cleanup hook**

Create `internal/lifecycle/cleanup_windows.go`:
```go
//go:build windows

package lifecycle

import "golang.org/x/sys/windows"

var (
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procSetConsoleCtrlHK  = kernel32.NewProc("SetConsoleCtrlHandler")
)

// OnConsoleClose runs handler when the console receives Ctrl+C / Ctrl+Break /
// close / logoff / shutdown. No-op effect if the process has no console (e.g.
// a -H=windowsgui release build); the hotkey and startup-reset paths remain.
func OnConsoleClose(handler func()) {
	cb := windows.NewCallback(func(ctrlType uint32) uintptr {
		handler()
		return 1
	})
	procSetConsoleCtrlHK.Call(cb, 1)
}
```

- [ ] **Step 4: Verify it builds**

Run: `go build ./...`
Expected: builds with no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/lifecycle/
git commit -m "feat: single-instance, hotkey loop, and console cleanup"
```

---

### Task 9: `main.go` — flags, config file, autostart, run loop

**Files:**
- Create: `cmd/giant-cursor/main.go`

**Interfaces:**
- Consumes: `cursor.NewWin32`, `input.NewWin32Poller`, `config.ShakeConfig`, `shake.New`, `app.New`, all `lifecycle` functions.

- [ ] **Step 1: Write main**

Create `cmd/giant-cursor/main.go`:
```go
// Command giant-cursor enlarges the Windows cursor when the mouse is shaken.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"giant-cursor/internal/app"
	"giant-cursor/internal/config"
	"giant-cursor/internal/cursor"
	"giant-cursor/internal/input"
	"giant-cursor/internal/lifecycle"
	"giant-cursor/internal/shake"

	"golang.org/x/sys/windows/registry"
)

const (
	appName      = "GiantCursor"
	mutexName    = "Global\\GiantCursorSingleInstance"
	runKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	pollInterval = 8 * time.Millisecond
)

type settings struct {
	Scale       int    `json:"scale"`
	Sensitivity string `json:"sensitivity"`
	IdleMillis  int64  `json:"idle_ms"`
}

func configPath() string {
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "giant-cursor")
	return filepath.Join(dir, "config.json")
}

func loadSettings(def settings) settings {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return def
	}
	var s settings
	if json.Unmarshal(data, &s) != nil {
		return def
	}
	if s.Scale < 1 {
		s.Scale = def.Scale
	}
	if s.Sensitivity == "" {
		s.Sensitivity = def.Sensitivity
	}
	if s.IdleMillis <= 0 {
		s.IdleMillis = def.IdleMillis
	}
	return s
}

func saveSettings(s settings) error {
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0o644)
}

func setAutostart(enabled bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !enabled {
		err := k.DeleteValue(appName)
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return k.SetStringValue(appName, fmt.Sprintf("%q --silent", exe))
}

func main() {
	scale := flag.Int("scale", 4, "cursor enlargement factor")
	sens := flag.String("sensitivity", "medium", "shake sensitivity: low|medium|high")
	idle := flag.Int64("idle-ms", 1000, "idle milliseconds before shrinking back")
	install := flag.Bool("install", false, "enable autostart and save settings, then run")
	uninstall := flag.Bool("uninstall", false, "disable autostart, restore cursors, and exit")
	restore := flag.Bool("restore", false, "restore cursors and exit (panic button)")
	_ = flag.Bool("silent", false, "reserved: run without console output")
	flag.Parse()

	cur := cursor.NewWin32(*scale)

	// Panic button and uninstall must work even if another instance is running.
	if *restore {
		_ = cur.Restore()
		return
	}
	if *uninstall {
		_ = setAutostart(false)
		_ = cur.Restore()
		return
	}

	release, already, err := lifecycle.AcquireSingleInstance(mutexName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "single-instance check failed:", err)
		return
	}
	if already {
		fmt.Println("Giant Cursor is already running.")
		return
	}
	defer release()

	s := loadSettings(settings{Scale: *scale, Sensitivity: *sens, IdleMillis: *idle})
	// Explicit flags override the config file.
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "scale":
			s.Scale = *scale
		case "sensitivity":
			s.Sensitivity = *sens
		case "idle-ms":
			s.IdleMillis = *idle
		}
	})
	cur = cursor.NewWin32(s.Scale)

	if *install {
		if err := saveSettings(s); err != nil {
			fmt.Fprintln(os.Stderr, "save settings failed:", err)
		}
		if err := setAutostart(true); err != nil {
			fmt.Fprintln(os.Stderr, "autostart failed:", err)
		}
	}

	// SAFETY: clean baseline first, so a previous crash that left the cursor
	// enlarged is fixed simply by launching again.
	_ = cur.Restore()

	det := shake.New(config.ShakeConfig(config.Sensitivity(s.Sensitivity), s.IdleMillis))
	application := app.New(det, cur)
	poller := input.NewWin32Poller()

	done := make(chan struct{})
	cleanup := func() { _ = cur.Restore() }
	lifecycle.OnConsoleClose(cleanup)

	go runLoop(application, poller, done)

	runtime.LockOSThread()
	lifecycle.RunHotkeyLoop(func() {
		close(done)
		cleanup()
	})
}

func runLoop(a *app.App, poller input.PositionSource, done <-chan struct{}) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	start := time.Now()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			pos, err := poller.Poll()
			if err != nil {
				continue
			}
			_ = a.Step(shake.Sample{Pos: pos, Millis: time.Since(start).Milliseconds()})
		}
	}
}
```

- [ ] **Step 2: Add the registry dependency and tidy**

Run:
```bash
go mod tidy
go build ./...
```
Expected: `golang.org/x/sys/windows/registry` resolves (same module, already present); build succeeds.

- [ ] **Step 3: Build the dev binary (with console for logs)**

Run:
```bash
go build -o giant-cursor.exe ./cmd/giant-cursor
```
Expected: produces `giant-cursor.exe`.

- [ ] **Step 4: Manual end-to-end verification**

1. Run `./giant-cursor.exe` in a terminal.
2. Shake the mouse briskly back and forth → the cursor should jump to ~4x size.
3. Keep moving → it stays large. Stop for ~1s → it returns to normal.
4. Press **Ctrl+Alt+Q** → the process exits and the cursor is normal.
5. Run `./giant-cursor.exe --restore` at any time → cursors reset.
6. Start two instances → the second prints "already running" and exits.

Expected: all six behaviours hold. If the shake is too hard/easy to trigger, note it for threshold tuning (Task 4 values); do not block the commit.

- [ ] **Step 5: Commit**

```bash
git add cmd/ go.mod go.sum
git commit -m "feat: wire main with flags, config, autostart, and run loop"
```

---

### Task 10: Inno Setup installer

**Files:**
- Create: `installer/giant-cursor.iss`

- [ ] **Step 1: Write the installer script**

Create `installer/giant-cursor.iss`:
```ini
[Setup]
AppName=Giant Cursor
AppVersion=1.0.0
AppPublisher=Giant Cursor
DefaultDirName={autopf}\Giant Cursor
DefaultGroupName=Giant Cursor
UninstallDisplayIcon={app}\giant-cursor.exe
OutputBaseFilename=GiantCursorSetup
Compression=lzma2
SolidCompression=yes
PrivilegesRequired=lowest
ArchitecturesInstallIn64BitMode=x64compatible

[Files]
Source: "..\giant-cursor.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\Giant Cursor"; Filename: "{app}\giant-cursor.exe"
Name: "{group}\Restore Cursor"; Filename: "{app}\giant-cursor.exe"; Parameters: "--restore"
Name: "{group}\Uninstall Giant Cursor"; Filename: "{uninstallexe}"

[Tasks]
Name: "startup"; Description: "Start Giant Cursor automatically when Windows starts"; GroupDescription: "Startup:"

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "GiantCursor"; ValueData: """{app}\giant-cursor.exe"" --silent"; Tasks: startup; Flags: uninsdeletevalue

[Run]
Filename: "{app}\giant-cursor.exe"; Description: "Launch Giant Cursor now"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "{app}\giant-cursor.exe"; Parameters: "--restore"; Flags: runhidden
```

- [ ] **Step 2: Verify (optional local build)**

If Inno Setup is installed, run: `iscc installer\giant-cursor.iss`
Expected: produces `installer/Output/GiantCursorSetup.exe`. (If Inno Setup is not installed locally, skip — CI builds it in Task 11.)

- [ ] **Step 3: Commit**

```bash
git add installer/
git commit -m "build: add Inno Setup installer script"
```

---

### Task 11: README, LICENSE, and release CI

**Files:**
- Create: `README.md`
- Create: `LICENSE`
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Write the README (English, public)**

Create `README.md`:
```markdown
# Giant Cursor

**Lose your mouse cursor? Shake it and it grows.**

Giant Cursor is a tiny, fast Windows utility for people with low vision. Shake
the mouse and the cursor instantly enlarges (4x by default) so you can find it,
then it shrinks back once you stop moving — just like macOS.

It is built to stay out of your way: no window, no tray icon, negligible CPU,
and it keeps working even when many apps are open.

## Why not PowerToys "Find My Mouse"?

PowerToys draws a full-screen spotlight (heavy) and relies on a global mouse
hook that Windows can starve when the system is busy — so it sometimes stops
responding. Giant Cursor instead **polls** the cursor position (which can't be
starved) and simply swaps the system cursor for a larger one — no overlay, no
per-frame drawing. It is dramatically lighter and more reliable under load.

## Install

**Option A — Installer (recommended)**
Download `GiantCursorSetup.exe` from the [Releases](../../releases) page and run
it. Tick "Start automatically when Windows starts" if you want it always on.

**Option B — Portable**
Download `giant-cursor.exe` and run it. That's it — no dependencies.

## Usage

- **Find the cursor:** shake the mouse back and forth. It grows to 4x.
- **Back to normal:** stop moving for ~1 second.
- **Quit cleanly:** press **Ctrl+Alt+Q** (this restores the cursor).
- **Panic button:** if a cursor ever gets stuck large, run
  `giant-cursor.exe --restore`, or just launch the app again (it resets on
  startup).

### Options

| Flag | Default | Description |
|------|---------|-------------|
| `--scale` | `4` | Enlargement factor. |
| `--sensitivity` | `medium` | `low`, `medium`, or `high`. |
| `--idle-ms` | `1000` | Idle time (ms) before shrinking back. |
| `--install` | — | Enable autostart and save your settings. |
| `--uninstall` | — | Disable autostart and restore the cursor. |
| `--restore` | — | Restore cursors and exit. |

Example: `giant-cursor.exe --scale 5 --sensitivity high --install`

## How it works

- A background goroutine polls `GetCursorPos` ~120 times/second and detects a
  shake as several rapid direction reversals within a short window.
- On a shake it calls `SetSystemCursor` to replace the standard system cursors
  with enlarged copies; after ~1s of no movement it reloads your cursor scheme
  with `SystemParametersInfo(SPI_SETCURSORS)`.

## Build from source

Requires Go 1.26+.

```bash
git clone <this-repo>
cd giant-cursor
go build -o giant-cursor.exe ./cmd/giant-cursor
```

Release build (no console window):

```bash
go build -ldflags "-H=windowsgui -s -w" -o giant-cursor.exe ./cmd/giant-cursor
```

Run the tests (the shake-detection logic is fully unit-tested):

```bash
go test ./...
```

## Known limitation

Only standard system cursors are enlarged. Apps that use a fully custom cursor
(some games/design tools) will not be affected. At 4x the scaled cursor looks
slightly pixelated — this is an accepted trade-off; crisp high-resolution
cursor assets may ship in a later version.

## License

MIT — see [LICENSE](LICENSE).
```

- [ ] **Step 2: Add a LICENSE**

Create `LICENSE` with the standard MIT text (year `2026`, holder `Giant Cursor contributors`).

- [ ] **Step 3: Write the release workflow**

Create `.github/workflows/release.yml`:
```yaml
name: release

on:
  push:
    tags: ["v*"]

permissions:
  contents: write

jobs:
  build:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"

      - name: Test
        run: go test ./...

      - name: Build portable exe
        run: go build -ldflags "-H=windowsgui -s -w" -o giant-cursor.exe ./cmd/giant-cursor

      - name: Install Inno Setup
        run: choco install innosetup -y

      - name: Build installer
        run: iscc installer\giant-cursor.iss

      - name: Publish release
        uses: softprops/action-gh-release@v2
        with:
          files: |
            giant-cursor.exe
            installer/Output/GiantCursorSetup.exe
```

- [ ] **Step 4: Verify tests still pass**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add README.md LICENSE .github/
git commit -m "docs: add README, license, and release workflow"
```

---

## Self-Review

**Spec coverage:**
- Detection by polling → Tasks 7, 9 (run loop). ✅
- Enlarge via `SetSystemCursor`, restore via `SPI_SETCURSORS` → Task 6. ✅
- State machine NORMAL→BIG→NORMAL → Tasks 2, 3, 5. ✅
- Idle-based shrink (~1s) → Task 3. ✅
- Default 4x, configurable → Task 6 (`NewWin32`), Task 9 (flags). ✅
- Hexagonal / pure domain / TDD → Tasks 1–5. ✅
- Safety (startup reset, `--restore`, console/session hooks, single instance) → Tasks 8, 9. ✅
- Config flags + `config.json` → Task 9. ✅
- Distribution: portable exe + Inno Setup + CI → Tasks 9, 10, 11. ✅
- Public English README → Task 11. ✅
- Sensitivity presets → Task 4. ✅

**Deferred (documented, not gaps):** exact per-sensitivity thresholds are starting values to be tuned during Task 9 manual verification (Design §11). High-resolution crisp cursor assets are explicitly v1.1 (Design §10).

**Placeholder scan:** No `TODO`/`TBD`/"handle edge cases" placeholders; every code step shows complete code. The `--silent` flag is intentionally reserved (declared, no behaviour) and documented as such.

**Type consistency:** `Enlarge()/Restore()` match between `cursor.Enlarger` (Task 5), `Win32` (Task 6), and `fakeEnlarger` (Task 5 test). `Poll() (shake.Point, error)` matches between `input.PositionSource` (Task 5) and `Win32Poller` (Task 7). `shake.Config` field names match across Tasks 1–5. `app.New(det, cur)` / `App.Step` signatures match between Task 5 impl and test, and Task 9 usage.

## Notes for the implementer

- Windows adapters (`*_windows.go`) only compile on Windows. On another OS, `go test ./...` still runs the pure domain (`shake`, `config`, `app`) because those files carry no build tag.
- The message loop in `RunHotkeyLoop` must run on the goroutine that called `runtime.LockOSThread()` — that is why `main` calls it last on the main goroutine.
- Release builds use `-H=windowsgui` (no console window); the console-close hook then becomes a no-op, and clean exit relies on the hotkey plus the startup reset and `--restore` safety nets.
