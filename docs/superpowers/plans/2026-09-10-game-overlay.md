# Game Overlay (beta) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Draw a click-through, always-on-top pointer aid so the cursor stays findable inside windowed games, which ignore `SetSystemCursor` because they ship their own cursor art.

**Architecture:** The overlay is a second `cursor.Enlarger` composed with the existing system-cursor effector through a new `cursor.Multi`, so both run together. A new optional `cursor.Tracker` interface carries the pointer position on every sample without widening `Enlarger`. The bitmap is painted once per activation and the window is only moved after that. The shape sits behind a `Painter` interface with two implementations.

**Tech Stack:** Go 1.26.2, `golang.org/x/sys/windows` for Win32 calls, `golang.org/x/image/draw` for scaling. Standard library `testing` only. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-09-10-game-overlay-design.md`

## Global Constraints

- **Never inject into another process.** No hooking, no DLL injection, no reading another process's memory. World of Warcraft runs Warden. This is not revisitable.
- **Test command:** `go test ./...` then `go vet ./...`. Both must pass before every commit.
- **Windows-only files** use the `//go:build windows` constraint and are only compiled and vetted on Windows. Anything worth testing goes in an untagged file.
- **Tests use the standard library only.** No testify, no mocking framework. Hand-written fakes named `fake<Interface>` with plain counter fields, pointer receivers, following `internal/app/app_test.go`.
- **No table-driven tests, no `t.Run`, no `t.Parallel`.** One behaviour per `TestXxx` function, plain `if` conditions, `t.Fatalf`/`t.Errorf`, matching the existing suite.
- **Artifact language is English.** Code, comments, identifiers, commit messages and docs.
- **Conventional commits, no AI attribution lines.**
- **Overlay default is `off`.** Unknown or empty stored values fall back to `off`.
- **The beta marker is user-visible** in the tray menu label, in both languages.
- Branch: `feat/game-overlay`.

---

### Task 1: Expose the arrow artwork as an image

`internal/cursor/art.go` can only produce Windows BGRA byte buffers today. The overlay's pointer painter needs an `image.RGBA` and the hotspot. Extract the scaling core so both callers share it.

**Files:**
- Modify: `internal/cursor/art.go:73-107` (the `bitmap` method)
- Test: `internal/cursor/art_test.go` (append)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `func cursor.ArrowImage(size int) (img *image.RGBA, hotX, hotY int)`, and the unexported `func (a *artwork) render(size int) (*image.RGBA, int, int)`.

- [ ] **Step 1: Write the failing test**

Append to `internal/cursor/art_test.go`:

```go
func TestArrowImage(t *testing.T) {
	const size = 128
	img, hotX, hotY := ArrowImage(size)

	if b := img.Bounds(); b.Dx() != size || b.Dy() != size {
		t.Fatalf("bounds = %dx%d, want %dx%d", b.Dx(), b.Dy(), size, size)
	}
	if _, _, _, a := img.At(hotX, hotY).RGBA(); a == 0 {
		t.Errorf("hotspot (%d,%d) sits on a transparent pixel", hotX, hotY)
	}
	if _, _, _, a := img.At(size-1, size-1).RGBA(); a != 0 {
		t.Errorf("bottom-right corner should be outside the artwork, got alpha=%d", a)
	}
}

func TestArrowImageMatchesBitmapHotspot(t *testing.T) {
	const size = 96
	_, wantX, wantY := arrowArt.bitmap(size)
	_, gotX, gotY := ArrowImage(size)

	if gotX != wantX || gotY != wantY {
		t.Errorf("ArrowImage hotspot = (%d,%d), bitmap hotspot = (%d,%d)", gotX, gotY, wantX, wantY)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/cursor/ -run TestArrowImage -v`
Expected: FAIL, compile error `undefined: ArrowImage`.

- [ ] **Step 3: Extract `render` and add `ArrowImage`**

In `internal/cursor/art.go`, replace the whole `bitmap` method with these three declarations:

```go
// render scales the artwork into a size×size premultiplied RGBA image anchored
// at the top-left, and returns it with the hotspot in destination pixels.
func (a *artwork) render(size int) (*image.RGBA, int, int) {
	if size < 1 {
		size = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	a.once.Do(a.load)
	if a.img == nil {
		return dst, 0, 0
	}

	sb := a.img.Bounds()
	targetH := int(float64(size) * artFill)
	if targetH < 1 {
		targetH = 1
	}
	targetW := targetH * sb.Dx() / sb.Dy()
	if targetW < 1 {
		targetW = 1
	}
	if targetW > size {
		targetW = size
	}

	xdraw.CatmullRom.Scale(dst, image.Rect(0, 0, targetW, targetH), a.img, sb, xdraw.Over, nil)

	s := float64(targetH) / float64(sb.Dy())
	return dst, int(float64(a.tipX) * s), int(float64(a.tipY) * s)
}

// bitmap renders the artwork into a size×size top-down 32bpp premultiplied BGRA
// buffer, anchored at the top-left, and returns the hotspot.
func (a *artwork) bitmap(size int) (buf []byte, hotX, hotY int) {
	if size < 1 {
		size = 1
	}
	dst, hotX, hotY := a.render(size)

	// image.RGBA is premultiplied RGBA; Windows wants premultiplied BGRA.
	buf = make([]byte, size*size*4)
	for i := 0; i < size*size; i++ {
		buf[i*4+0] = dst.Pix[i*4+2] // B
		buf[i*4+1] = dst.Pix[i*4+1] // G
		buf[i*4+2] = dst.Pix[i*4+0] // R
		buf[i*4+3] = dst.Pix[i*4+3] // A
	}
	return buf, hotX, hotY
}

// ArrowImage renders the crisp arrow artwork at the given cursor size and
// returns it with its tip position. The image is size×size premultiplied RGBA
// with the arrow anchored at the top-left.
func ArrowImage(size int) (img *image.RGBA, hotX, hotY int) {
	return arrowArt.render(size)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/cursor/ -v` then `go vet ./internal/cursor/`
Expected: PASS, including the pre-existing `TestArtworkBitmaps` and `TestArtworkScalesWithSize`, which prove the refactor did not change `bitmap`'s output.

- [ ] **Step 5: Commit**

```bash
git add internal/cursor/art.go internal/cursor/art_test.go
git commit -m "refactor: expose the arrow artwork as an image"
```

---

### Task 2: The Tracker port and the Multi composer

**Files:**
- Modify: `internal/cursor/cursor.go` (append after the `Enlarger` interface)
- Create: `internal/cursor/multi_test.go`

**Interfaces:**
- Consumes: `cursor.Enlarger` (existing).
- Produces: `type cursor.Tracker interface { Track(x, y int) }` and `type cursor.Multi []Enlarger` with methods `Enlarge() error`, `Restore() error`, `Track(x, y int)`.

- [ ] **Step 1: Write the failing test**

Create `internal/cursor/multi_test.go`:

```go
package cursor

import (
	"errors"
	"testing"
)

type fakeEffector struct {
	enlarged, restored int
	err                error
}

func (f *fakeEffector) Enlarge() error { f.enlarged++; return f.err }
func (f *fakeEffector) Restore() error { f.restored++; return f.err }

type fakeTrackingEffector struct {
	fakeEffector
	tracked      int
	lastX, lastY int
}

func (f *fakeTrackingEffector) Track(x, y int) { f.tracked++; f.lastX, f.lastY = x, y }

func TestMultiFansOutToEveryMember(t *testing.T) {
	a, b := &fakeEffector{}, &fakeEffector{}
	m := Multi{a, b}

	if err := m.Enlarge(); err != nil {
		t.Fatalf("Enlarge = %v, want nil", err)
	}
	if err := m.Restore(); err != nil {
		t.Fatalf("Restore = %v, want nil", err)
	}
	if a.enlarged != 1 || b.enlarged != 1 {
		t.Errorf("Enlarge reached members %d and %d times, want 1 and 1", a.enlarged, b.enlarged)
	}
	if a.restored != 1 || b.restored != 1 {
		t.Errorf("Restore reached members %d and %d times, want 1 and 1", a.restored, b.restored)
	}
}

func TestMultiAttemptsEveryMemberAfterAnError(t *testing.T) {
	boom := errors.New("boom")
	failing, healthy := &fakeEffector{err: boom}, &fakeEffector{}
	m := Multi{failing, healthy}

	if err := m.Enlarge(); !errors.Is(err, boom) {
		t.Fatalf("Enlarge = %v, want boom", err)
	}
	if healthy.enlarged != 1 {
		t.Error("a failing member must not stop the members after it")
	}
}

func TestMultiReturnsTheFirstError(t *testing.T) {
	first, second := errors.New("first"), errors.New("second")
	m := Multi{&fakeEffector{err: first}, &fakeEffector{err: second}}

	if err := m.Restore(); !errors.Is(err, first) {
		t.Fatalf("Restore = %v, want the first error", err)
	}
}

func TestMultiTracksOnlyTrackingMembers(t *testing.T) {
	plain, tracker := &fakeEffector{}, &fakeTrackingEffector{}
	m := Multi{plain, tracker}

	m.Track(120, 340)

	if tracker.tracked != 1 {
		t.Fatalf("tracked %d times, want 1", tracker.tracked)
	}
	if tracker.lastX != 120 || tracker.lastY != 340 {
		t.Errorf("tracked (%d,%d), want (120,340)", tracker.lastX, tracker.lastY)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/cursor/ -run TestMulti -v`
Expected: FAIL, compile error `undefined: Multi`.

- [ ] **Step 3: Write the implementation**

Append to `internal/cursor/cursor.go`:

```go
// Tracker is an optional Enlarger extension for effects that follow the pointer
// while enlarged, such as the game overlay. Effectors that do not implement it
// are driven exactly as before. It takes plain coordinates so this package
// gains no dependency on the shake detector.
type Tracker interface {
	Track(x, y int)
}

// Multi fans Enlarge and Restore out to several effectors, so the system cursor
// swap and the game overlay can run side by side.
type Multi []Enlarger

// Enlarge enlarges every member. Every member is attempted even if an earlier
// one fails, so one broken effector never blocks the others; the first error is
// returned.
func (m Multi) Enlarge() error { return m.each(Enlarger.Enlarge) }

// Restore restores every member, with the same all-members-attempted rule as
// Enlarge. Restore must be total: a stuck enlarged cursor is the worst failure
// this app can produce.
func (m Multi) Restore() error { return m.each(Enlarger.Restore) }

func (m Multi) each(op func(Enlarger) error) error {
	var first error
	for _, e := range m {
		if err := op(e); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Track forwards the pointer position to every member that implements Tracker.
func (m Multi) Track(x, y int) {
	for _, e := range m {
		if t, ok := e.(Tracker); ok {
			t.Track(x, y)
		}
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/cursor/ -v` then `go vet ./internal/cursor/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cursor/cursor.go internal/cursor/multi_test.go
git commit -m "feat: add the Tracker port and the Multi effector composer"
```

---

### Task 3: Feed the pointer position through the app

**Files:**
- Modify: `internal/app/app.go` (the `App` struct, `New`, and `Step`)
- Test: `internal/app/app_test.go` (append)

**Interfaces:**
- Consumes: `cursor.Tracker` from Task 2.
- Produces: `app.App` now calls `Track(x, y)` on its effector while the state is `shake.StateBig`, including on the sample that triggers enlargement.

- [ ] **Step 1: Write the failing test**

Append to `internal/app/app_test.go`:

```go
type fakeTrackingEnlarger struct {
	fakeEnlarger
	tracked      int
	lastX, lastY int
}

func (f *fakeTrackingEnlarger) Track(x, y int) { f.tracked++; f.lastX, f.lastY = x, y }

func TestTracksOnlyWhileEnlarged(t *testing.T) {
	fe := &fakeTrackingEnlarger{}
	det := shake.New(shake.Config{
		WindowMillis: 500, MinReversals: 4, NoiseFloor: 3,
		HoldMillis: 1000,
	})
	a := New(det, fe)

	// A single calm sample must not enlarge, and must not track.
	_ = a.Step(shake.Sample{Pos: shake.Point{X: 5}, Millis: 0})
	if fe.tracked != 0 {
		t.Fatalf("tracked %d times while normal, want 0", fe.tracked)
	}

	drive(a, []int32{0, 30, 0, 30, 0, 30, 0}, 20, 20)
	if fe.enlarged != 1 {
		t.Fatalf("want 1 Enlarge, got %d", fe.enlarged)
	}
	if fe.tracked == 0 {
		t.Fatal("the effector must be tracked while enlarged")
	}

	// The last sample of the drive above sat at X=0, Y=0.
	if fe.lastX != 0 || fe.lastY != 0 {
		t.Errorf("last tracked position = (%d,%d), want (0,0)", fe.lastX, fe.lastY)
	}

	before := fe.tracked
	_ = a.Step(shake.Sample{Pos: shake.Point{X: 0}, Millis: 10000}) // idle: BIG->NORMAL
	if fe.restored != 1 {
		t.Fatalf("want 1 Restore after idle, got %d", fe.restored)
	}
	if fe.tracked != before {
		t.Errorf("tracked %d times after restore, want it to stop at %d", fe.tracked, before)
	}
}

func TestPlainEnlargerIsDrivenWithoutTracking(t *testing.T) {
	fe := &fakeEnlarger{}
	det := shake.New(shake.Config{
		WindowMillis: 500, MinReversals: 4, NoiseFloor: 3,
		HoldMillis: 1000,
	})
	a := New(det, fe)

	drive(a, []int32{0, 30, 0, 30, 0, 30, 0}, 20, 0)

	if fe.enlarged != 1 {
		t.Fatalf("an effector without Track must still be enlarged, got %d", fe.enlarged)
	}
}

func TestTracksOnTheEnlargingSample(t *testing.T) {
	fe := &fakeTrackingEnlarger{}
	det := shake.New(shake.Config{
		WindowMillis: 500, MinReversals: 4, NoiseFloor: 3,
		HoldMillis: 1000,
	})
	a := New(det, fe)

	drive(a, []int32{0, 30, 0, 30, 0, 30, 0}, 20, 0)

	// The overlay must appear where the pointer is, never at the origin by
	// default, so the enlarging sample itself has to be tracked.
	if fe.tracked == 0 {
		t.Fatal("the sample that triggers enlargement must also be tracked")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/app/ -v`
Expected: FAIL on `TestTracksOnlyWhileEnlarged` with `tracked 0 times` after the drive, because nothing calls `Track` yet.

- [ ] **Step 3: Write the implementation**

Replace the `App` struct, `New`, and `Step` in `internal/app/app.go`:

```go
// App drives the enlarge/restore side effects from detector state changes.
type App struct {
	det   *shake.Detector
	cur   cursor.Enlarger
	trk   cursor.Tracker // nil unless the effector follows the pointer
	state shake.State
}

// New returns an App in the NORMAL state. If the effector also implements
// cursor.Tracker it receives the pointer position on every enlarged sample.
// The assertion happens once here, never per sample.
func New(det *shake.Detector, cur cursor.Enlarger) *App {
	a := &App{det: det, cur: cur, state: shake.StateNormal}
	if t, ok := cur.(cursor.Tracker); ok {
		a.trk = t
	}
	return a
}

// Step feeds one sample to the detector and applies the resulting side effect.
func (a *App) Step(s shake.Sample) error {
	prev := a.state
	next := a.det.Update(s)
	a.state = next
	switch {
	case prev == shake.StateNormal && next == shake.StateBig:
		err := a.cur.Enlarge()
		a.track(s) // place the overlay before the user can see it
		return err
	case prev == shake.StateBig && next == shake.StateNormal:
		return a.cur.Restore()
	case next == shake.StateBig:
		a.track(s)
	}
	return nil
}

// track forwards the pointer position to a tracking effector, if there is one.
func (a *App) track(s shake.Sample) {
	if a.trk != nil {
		a.trk.Track(int(s.Pos.X), int(s.Pos.Y))
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/app/ -v` then `go vet ./internal/app/`
Expected: PASS, including the pre-existing `TestEnlargeOnShakeRestoreOnIdle`.

- [ ] **Step 5: Commit**

```bash
git add internal/app/app.go internal/app/app_test.go
git commit -m "feat: forward the pointer position to tracking effectors"
```

---

### Task 4: The overlay mode setting

**Files:**
- Create: `internal/config/overlay.go`
- Test: `internal/config/overlay_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `type config.OverlayMode string` with constants `config.OverlayOff` (`"off"`), `config.OverlayHalo` (`"halo"`), `config.OverlayPointer` (`"pointer"`), and `func config.ParseOverlayMode(v string) OverlayMode`.

- [ ] **Step 1: Write the failing test**

Create `internal/config/overlay_test.go`:

```go
package config

import "testing"

func TestParseOverlayModeKnownValues(t *testing.T) {
	if ParseOverlayMode("halo") != OverlayHalo {
		t.Error("halo should parse to OverlayHalo")
	}
	if ParseOverlayMode("pointer") != OverlayPointer {
		t.Error("pointer should parse to OverlayPointer")
	}
	if ParseOverlayMode("off") != OverlayOff {
		t.Error("off should parse to OverlayOff")
	}
}

func TestParseOverlayModeDefaultsToOff(t *testing.T) {
	if ParseOverlayMode("") != OverlayOff {
		t.Error("an empty value should default to off, so a config file without the key stays off")
	}
	if ParseOverlayMode("bogus") != OverlayOff {
		t.Error("an unknown value should default to off")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/config/ -v`
Expected: FAIL, compile error `undefined: ParseOverlayMode`.

- [ ] **Step 3: Write the implementation**

Create `internal/config/overlay.go`:

```go
package config

// OverlayMode selects what the game overlay draws, if anything. Games that ship
// their own cursor art ignore SetSystemCursor, so the overlay is the only way to
// make the pointer findable inside them.
type OverlayMode string

const (
	// OverlayOff draws nothing and creates no overlay window at all.
	OverlayOff OverlayMode = "off"
	// OverlayHalo draws a ring around the pointer. It annotates rather than
	// replaces, so it does not compete with the cursor the game draws itself.
	OverlayHalo OverlayMode = "halo"
	// OverlayPointer draws an enlarged arrow at the pointer. The game keeps
	// drawing its own, so the user sees two cursors: expected in this mode.
	OverlayPointer OverlayMode = "pointer"
)

// ParseOverlayMode maps a stored value to a mode. Anything unknown or empty
// falls back to OverlayOff, so a config file written before this feature
// existed keeps the overlay switched off.
func ParseOverlayMode(v string) OverlayMode {
	switch OverlayMode(v) {
	case OverlayHalo:
		return OverlayHalo
	case OverlayPointer:
		return OverlayPointer
	default:
		return OverlayOff
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/config/ -v` then `go vet ./internal/config/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/overlay.go internal/config/overlay_test.go
git commit -m "feat: add the overlay mode setting"
```

---

### Task 5: The painters

The window is the expensive part, not the shape, so both shapes cost almost the same as one. These files carry no build constraint, so they compile and are tested on every platform.

**Files:**
- Create: `internal/overlay/painter.go`
- Create: `internal/overlay/halo.go`
- Create: `internal/overlay/pointer.go`
- Test: `internal/overlay/painter_test.go`

**Interfaces:**
- Consumes: `cursor.ArrowImage` from Task 1.
- Produces: `type overlay.Painter interface { Size(cursorSize int) int; Draw(cursorSize int) (*image.RGBA, int, int) }`, `overlay.Halo{}`, `overlay.Pointer{}`, and `func overlay.PainterFor(mode config.OverlayMode) Painter`.

- [ ] **Step 1: Write the failing test**

Create `internal/overlay/painter_test.go`:

```go
package overlay

import (
	"math"
	"testing"

	"github.com/JorgeLBJ/giant-cursor/internal/config"
)

func alphaAt(p Painter, cursorSize, x, y int) uint32 {
	img, _, _ := p.Draw(cursorSize)
	_, _, _, a := img.At(x, y).RGBA()
	return a
}

func TestHaloIsAHollowRing(t *testing.T) {
	const cursorSize = 64
	h := Halo{}
	size := h.Size(cursorSize)
	img, ax, ay := h.Draw(cursorSize)

	if b := img.Bounds(); b.Dx() != size || b.Dy() != size {
		t.Fatalf("bounds = %dx%d, want %dx%d", b.Dx(), b.Dy(), size, size)
	}
	if ax != size/2 || ay != size/2 {
		t.Errorf("anchor = (%d,%d), want the centre (%d,%d)", ax, ay, size/2, size/2)
	}
	if a := alphaAt(h, cursorSize, size/2, size/2); a != 0 {
		t.Errorf("the centre must stay hollow so the game is visible through it, got alpha=%d", a)
	}
	if a := alphaAt(h, cursorSize, 0, 0); a != 0 {
		t.Errorf("the corner is outside the circle, got alpha=%d", a)
	}
}

func TestHaloDrawsOnItsRadius(t *testing.T) {
	const cursorSize = 64
	h := Halo{}
	size := h.Size(cursorSize)

	// A point just inside the outer edge, on the horizontal axis.
	x := size - 1 - int(math.Round(float64(size)*haloThickness/2))
	if a := alphaAt(h, cursorSize, x, size/2); a == 0 {
		t.Errorf("expected the ring to be drawn at x=%d, got a transparent pixel", x)
	}
}

func TestHaloIsBiggerThanTheCursor(t *testing.T) {
	if got := (Halo{}).Size(64); got <= 64 {
		t.Errorf("halo size = %d, it must surround a 64px cursor", got)
	}
}

func TestPointerDrawsTheArrowAtItsHotspot(t *testing.T) {
	const cursorSize = 96
	p := Pointer{}

	if got := p.Size(cursorSize); got != cursorSize {
		t.Errorf("pointer size = %d, want %d", got, cursorSize)
	}
	img, ax, ay := p.Draw(cursorSize)
	if _, _, _, a := img.At(ax, ay).RGBA(); a == 0 {
		t.Errorf("anchor (%d,%d) sits on a transparent pixel, so the arrow tip would not follow the pointer", ax, ay)
	}
}

func TestPainterForMapsEveryMode(t *testing.T) {
	if _, ok := PainterFor(config.OverlayHalo).(Halo); !ok {
		t.Error("halo mode should select the Halo painter")
	}
	if _, ok := PainterFor(config.OverlayPointer).(Pointer); !ok {
		t.Error("pointer mode should select the Pointer painter")
	}
	if PainterFor(config.OverlayOff) != nil {
		t.Error("off mode must select no painter at all")
	}
}

func TestPaintersSurviveATinyCursor(t *testing.T) {
	for _, p := range []Painter{Halo{}, Pointer{}} {
		if got := p.Size(0); got < 1 {
			t.Errorf("%T: size = %d for a zero cursor, want at least 1", p, got)
		}
		if img, _, _ := p.Draw(0); img.Bounds().Empty() {
			t.Errorf("%T: drew an empty image for a zero cursor", p)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/overlay/ -v`
Expected: FAIL, the package does not exist yet.

- [ ] **Step 3: Write the implementation**

Create `internal/overlay/painter.go`:

```go
// Package overlay draws a pointer aid on top of everything else. Games that
// ship their own cursor art ignore SetSystemCursor, so a separate always-on-top
// window is the only way to make the pointer findable inside them without
// injecting into the game, which anti-cheat systems treat as an attack.
package overlay

import (
	"image"

	"github.com/JorgeLBJ/giant-cursor/internal/config"
)

// Painter renders the overlay bitmap for a given enlarged cursor size. The
// window is the expensive part, not the shape, so swapping painters is how the
// user picks a look without paying for a second implementation.
type Painter interface {
	// Size returns the window edge length in pixels for a cursor size.
	Size(cursorSize int) int
	// Draw renders the art in premultiplied RGBA and returns the point inside
	// the image that must sit exactly on the pointer.
	Draw(cursorSize int) (img *image.RGBA, anchorX, anchorY int)
}

// PainterFor returns the painter for a mode, or nil for config.OverlayOff.
// A nil painter means no window is created at all.
func PainterFor(mode config.OverlayMode) Painter {
	switch mode {
	case config.OverlayHalo:
		return Halo{}
	case config.OverlayPointer:
		return Pointer{}
	default:
		return nil
	}
}
```

Create `internal/overlay/halo.go`:

```go
package overlay

import (
	"image"
	"image/color"
	"math"
)

// Starting geometry from the design document, to be tuned against real play.
// haloOuter is the ring diameter as a multiple of the enlarged cursor size;
// haloThickness is the ring thickness as a fraction of that diameter.
const (
	haloOuter     = 2.5
	haloThickness = 0.12
)

// Halo draws a ring centred on the pointer. It annotates instead of replacing,
// so it never competes with the cursor the game draws itself, and a thick soft
// shape hides the few pixels of lag between our window and the game's frame.
type Halo struct{}

// Size returns the ring's bounding box edge in pixels.
func (Halo) Size(cursorSize int) int {
	s := int(float64(cursorSize) * haloOuter)
	if s < 1 {
		s = 1
	}
	return s
}

// Draw renders the ring, anchored at its centre.
func (h Halo) Draw(cursorSize int) (*image.RGBA, int, int) {
	size := h.Size(cursorSize)
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	centre := float64(size) / 2
	outer := centre
	inner := outer - float64(size)*haloThickness
	if inner < 0 {
		inner = 0
	}
	// A dark border on both edges of the ring keeps it readable against both
	// bright snow and a dark dungeon.
	border := (outer - inner) / 3

	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - centre
			dy := float64(y) + 0.5 - centre
			d := math.Sqrt(dx*dx + dy*dy)
			switch {
			case d > outer || d < inner:
				continue
			case d > outer-border || d < inner+border:
				img.SetRGBA(x, y, black)
			default:
				img.SetRGBA(x, y, white)
			}
		}
	}
	return img, size / 2, size / 2
}
```

Create `internal/overlay/pointer.go`:

```go
package overlay

import (
	"image"

	"github.com/JorgeLBJ/giant-cursor/internal/cursor"
)

// Pointer draws an enlarged arrow at the pointer, reusing the same crisp
// artwork the system-cursor effector uses. The game keeps drawing its own
// cursor and we cannot stop it without injecting, so the user sees two
// pointers in this mode. That is expected behaviour, not a defect.
type Pointer struct{}

// Size returns the arrow's bounding box edge in pixels.
func (Pointer) Size(cursorSize int) int {
	if cursorSize < 1 {
		return 1
	}
	return cursorSize
}

// Draw renders the arrow, anchored at its tip.
func (p Pointer) Draw(cursorSize int) (*image.RGBA, int, int) {
	return cursor.ArrowImage(p.Size(cursorSize))
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/overlay/ -v` then `go vet ./internal/overlay/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/overlay/
git commit -m "feat: add the halo and pointer overlay painters"
```

---

### Task 6: The layered overlay window

This is the one task with no unit tests. The window is deliberately thin: every decision worth testing already lives in a painter. It is verified by `go vet` and by running the app.

The overlay is created **once for the process** and never rebuilt. `control.Controller.rebuild()` recreates the effector on every settings change, so an overlay constructed there would leak a Win32 window per menu click. Mode and scale changes swap the painter instead.

**Files:**
- Create: `internal/overlay/overlay_windows.go`
- Create: `internal/overlay/overlay_stub.go`

**Interfaces:**
- Consumes: `overlay.Painter` and `overlay.PainterFor` from Task 5, `config.OverlayMode` from Task 4.
- Produces: `func overlay.New() *Overlay`, and on `*Overlay` the methods `Enlarge() error`, `Restore() error`, `Track(x, y int)`, `SetMode(mode config.OverlayMode)`, `SetScale(cursorSize int)`. `*Overlay` satisfies both `cursor.Enlarger` and `cursor.Tracker`.

- [ ] **Step 1: Write the non-Windows stub**

The rest of the package builds everywhere, so `Overlay` needs a stub for other platforms. Create `internal/overlay/overlay_stub.go`:

```go
//go:build !windows

package overlay

import "github.com/JorgeLBJ/giant-cursor/internal/config"

// Overlay is a no-op outside Windows. The app only ships for Windows; this
// stub exists so the package still builds and its painters stay testable
// on any developer machine.
type Overlay struct{}

// New returns an inert overlay.
func New() *Overlay { return &Overlay{} }

// Enlarge does nothing.
func (o *Overlay) Enlarge() error { return nil }

// Restore does nothing.
func (o *Overlay) Restore() error { return nil }

// Track does nothing.
func (o *Overlay) Track(x, y int) {}

// SetMode does nothing.
func (o *Overlay) SetMode(mode config.OverlayMode) {}

// SetScale does nothing.
func (o *Overlay) SetScale(cursorSize int) {}
```

- [ ] **Step 2: Verify the package still builds and tests pass**

Run: `go test ./internal/overlay/ -v` then `go vet ./internal/overlay/`
Expected: PASS on Windows the stub is excluded, so this only proves nothing broke. Continue.

- [ ] **Step 3: Write the Windows implementation**

Create `internal/overlay/overlay_windows.go`:

```go
//go:build windows

package overlay

import (
	"fmt"
	"image"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"github.com/JorgeLBJ/giant-cursor/internal/config"
	"golang.org/x/sys/windows"
)

var (
	user32o             = windows.NewLazySystemDLL("user32.dll")
	procRegisterClassO  = user32o.NewProc("RegisterClassW")
	procCreateWindowExO = user32o.NewProc("CreateWindowExW")
	procDefWindowProcO  = user32o.NewProc("DefWindowProcW")
	procGetMessageO     = user32o.NewProc("GetMessageW")
	procTranslateMsgO   = user32o.NewProc("TranslateMessage")
	procDispatchMsgO    = user32o.NewProc("DispatchMessageW")
	procShowWindowO     = user32o.NewProc("ShowWindow")
	procSetWindowPosO   = user32o.NewProc("SetWindowPos")
	procUpdateLayeredW  = user32o.NewProc("UpdateLayeredWindow")
	procGetDCO          = user32o.NewProc("GetDC")
	procReleaseDCO      = user32o.NewProc("ReleaseDC")

	gdi32o              = windows.NewLazySystemDLL("gdi32.dll")
	procCreateCompatDCO = gdi32o.NewProc("CreateCompatibleDC")
	procDeleteDCO       = gdi32o.NewProc("DeleteDC")
	procCreateDIBSectO  = gdi32o.NewProc("CreateDIBSection")
	procSelectObjectO   = gdi32o.NewProc("SelectObject")
	procDeleteObjectO   = gdi32o.NewProc("DeleteObject")
)

const (
	wsExLayered     = 0x00080000
	wsExTransparent = 0x00000020
	wsExTopmost     = 0x00000008
	wsExToolWindow  = 0x00000080
	wsExNoActivate  = 0x08000000
	wsPopup         = 0x80000000

	swHide     = 0
	swShowNA   = 8 // show without activating: never steal focus from the game
	hwndTopmost = ^uintptr(0)  // (HWND)-1
	swpNoActivate = 0x0010
	swpNoZOrder   = 0x0004

	ulwAlpha       = 0x00000002
	acSrcOver      = 0x00
	acSrcAlpha     = 0x01
	biRGB          = 0
	dibRGBColors   = 0
)

// debugf reports overlay failures when GIANTCURSOR_DEBUG is set. The overlay is
// an enhancement: it must never take the app down with it.
func debugf(format string, a ...any) {
	if os.Getenv("GIANTCURSOR_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[giant-cursor overlay] "+format+"\n", a...)
	}
}

type wndClassO struct {
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
}

type msgO struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ X, Y int32 }
}

type bitmapInfoHeaderO struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type blendFunction struct {
	BlendOp             byte
	BlendFlags          byte
	SourceConstantAlpha byte
	AlphaFormat         byte
}

type pointO struct{ X, Y int32 }
type sizeO struct{ CX, CY int32 }

// Overlay is a click-through, always-on-top window that follows the pointer.
// It implements cursor.Enlarger and cursor.Tracker.
//
// It is created ONCE for the process. The controller rebuilds the cursor
// effector on every settings change, so an overlay created there would leak a
// window per menu click; mode and scale changes swap the painter instead.
type Overlay struct {
	mu      sync.Mutex
	painter Painter
	size    int  // enlarged cursor size in pixels
	visible bool
	failed  bool // a creation failure disables the overlay for the session

	hwnd    uintptr
	edge    int // current window edge length
	anchorX int
	anchorY int

	ready chan struct{}
	once  sync.Once
}

// New returns an overlay with no painter, so it does nothing until SetMode
// selects one. The window is created lazily on the first Enlarge with a
// painter set, so config.OverlayOff really does cost nothing.
func New() *Overlay {
	return &Overlay{ready: make(chan struct{})}
}

// SetMode selects the painter, or none at all for config.OverlayOff. Switching
// to off hides the window immediately; switching between shapes discards the
// cached bitmap, which is redrawn on the next Enlarge.
func (o *Overlay) SetMode(mode config.OverlayMode) {
	o.mu.Lock()
	o.painter = PainterFor(mode)
	o.mu.Unlock()
	if o.painter == nil {
		_ = o.Restore()
	}
}

// SetScale sets the enlarged cursor size the painters draw for.
func (o *Overlay) SetScale(cursorSize int) {
	o.mu.Lock()
	o.size = cursorSize
	o.mu.Unlock()
}

// Enlarge paints the overlay and shows it. The bitmap is built here, once per
// activation, so Track only has to move a window.
func (o *Overlay) Enlarge() error {
	o.mu.Lock()
	painter, size, failed := o.painter, o.size, o.failed
	o.mu.Unlock()

	if painter == nil || failed {
		return nil
	}
	if err := o.ensureWindow(); err != nil {
		o.mu.Lock()
		o.failed = true
		o.mu.Unlock()
		debugf("window creation failed, overlay disabled for this session: %v", err)
		return nil
	}

	img, ax, ay := painter.Draw(size)
	if err := o.paint(img, ax, ay); err != nil {
		debugf("paint failed: %v", err)
		return nil
	}

	o.mu.Lock()
	o.visible = true
	o.mu.Unlock()
	procShowWindowO.Call(o.hwnd, swShowNA)
	return nil
}

// Restore hides the overlay. The window and its bitmap are kept for reuse.
func (o *Overlay) Restore() error {
	o.mu.Lock()
	hwnd, visible := o.hwnd, o.visible
	o.visible = false
	o.mu.Unlock()

	if hwnd != 0 && visible {
		procShowWindowO.Call(hwnd, swHide)
	}
	return nil
}

// Track centres the window on the pointer. Nothing is repainted here: at 125
// samples per second, moving a window is cheap and repainting one is not.
func (o *Overlay) Track(x, y int) {
	o.mu.Lock()
	hwnd, visible, ax, ay := o.hwnd, o.visible, o.anchorX, o.anchorY
	o.mu.Unlock()

	if hwnd == 0 || !visible {
		return
	}
	procSetWindowPosO.Call(
		hwnd, 0,
		uintptr(int32(x-ax)), uintptr(int32(y-ay)), 0, 0,
		swpNoActivate|swpNoZOrder|0x0001, // SWP_NOSIZE
	)
}

// ensureWindow starts the overlay's own thread and message pump on first use.
// Win32 windows are thread-affine and the tray owns the only other pump, so the
// overlay keeps its own; that way its lifetime never entangles with the tray's.
func (o *Overlay) ensureWindow() error {
	var err error
	o.once.Do(func() {
		started := make(chan error, 1)
		go o.run(started)
		err = <-started
	})
	if err != nil {
		return err
	}
	if o.hwnd == 0 {
		return fmt.Errorf("overlay window was not created")
	}
	return nil
}

func (o *Overlay) run(started chan<- error) {
	runtime.LockOSThread()

	hinst, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	className, _ := windows.UTF16PtrFromString("GiantCursorOverlayWnd")

	wc := wndClassO{
		lpfnWndProc:   windows.NewCallback(overlayWndProc),
		hInstance:     hinst,
		lpszClassName: className,
	}
	if atom, _, e := procRegisterClassO.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		started <- fmt.Errorf("RegisterClass failed: %w", e)
		return
	}

	hwnd, _, e := procCreateWindowExO.Call(
		wsExLayered|wsExTransparent|wsExTopmost|wsExToolWindow|wsExNoActivate,
		uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(className)),
		wsPopup,
		0, 0, 1, 1,
		0, 0, hinst, 0,
	)
	if hwnd == 0 {
		started <- fmt.Errorf("CreateWindowEx failed: %w", e)
		return
	}
	o.hwnd = hwnd
	started <- nil

	var m msgO
	for {
		r, _, _ := procGetMessageO.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		procTranslateMsgO.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMsgO.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func overlayWndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	r, _, _ := procDefWindowProcO.Call(hwnd, msg, wparam, lparam)
	return r
}

// paint pushes an image into the layered window with per-pixel alpha.
func (o *Overlay) paint(img *image.RGBA, anchorX, anchorY int) error {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 1 || h < 1 {
		return fmt.Errorf("refusing to paint an empty image")
	}

	screenDC, _, _ := procGetDCO.Call(0)
	if screenDC == 0 {
		return fmt.Errorf("GetDC failed")
	}
	defer procReleaseDCO.Call(0, screenDC)

	memDC, _, _ := procCreateCompatDCO.Call(screenDC)
	if memDC == 0 {
		return fmt.Errorf("CreateCompatibleDC failed")
	}
	defer procDeleteDCO.Call(memDC)

	// Negative height makes the DIB top-down, matching image.RGBA's row order.
	bi := bitmapInfoHeaderO{
		Size: uint32(unsafe.Sizeof(bitmapInfoHeaderO{})),
		Width: int32(w), Height: int32(-h),
		Planes: 1, BitCount: 32, Compression: biRGB,
	}
	var bits unsafe.Pointer
	dib, _, _ := procCreateDIBSectO.Call(
		memDC, uintptr(unsafe.Pointer(&bi)), dibRGBColors,
		uintptr(unsafe.Pointer(&bits)), 0, 0,
	)
	if dib == 0 || bits == nil {
		return fmt.Errorf("CreateDIBSection failed")
	}
	defer procDeleteObjectO.Call(dib)

	// image.RGBA is premultiplied RGBA; Windows wants premultiplied BGRA.
	dst := unsafe.Slice((*byte)(bits), w*h*4)
	for i := 0; i < w*h; i++ {
		dst[i*4+0] = img.Pix[i*4+2] // B
		dst[i*4+1] = img.Pix[i*4+1] // G
		dst[i*4+2] = img.Pix[i*4+0] // R
		dst[i*4+3] = img.Pix[i*4+3] // A
	}

	old, _, _ := procSelectObjectO.Call(memDC, dib)
	defer procSelectObjectO.Call(memDC, old)

	size := sizeO{CX: int32(w), CY: int32(h)}
	src := pointO{}
	blend := blendFunction{BlendOp: acSrcOver, SourceConstantAlpha: 255, AlphaFormat: acSrcAlpha}

	r, _, e := procUpdateLayeredW.Call(
		o.hwnd, screenDC, 0,
		uintptr(unsafe.Pointer(&size)),
		memDC, uintptr(unsafe.Pointer(&src)),
		0, uintptr(unsafe.Pointer(&blend)), ulwAlpha,
	)
	if r == 0 {
		return fmt.Errorf("UpdateLayeredWindow failed: %w", e)
	}

	o.mu.Lock()
	o.edge, o.anchorX, o.anchorY = w, anchorX, anchorY
	o.mu.Unlock()
	// Keep the window on top: a game going fullscreen-windowed can push it down.
	procSetWindowPosO.Call(o.hwnd, hwndTopmost, 0, 0, 0, 0, swpNoActivate|0x0001|0x0002) // SWP_NOSIZE|SWP_NOMOVE
	return nil
}
```

- [ ] **Step 4: Verify it compiles and vets clean**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: builds, vets clean, all existing tests pass. There are no new tests in this task by design.

- [ ] **Step 5: Commit**

```bash
git add internal/overlay/overlay_windows.go internal/overlay/overlay_stub.go
git commit -m "feat: add the click-through layered overlay window"
```

---

### Task 7: Carry the overlay mode through the controller

`control.Controller` rebuilds the effector on every settings change. The overlay must survive those rebuilds, so `newEnlarger` takes the whole `Settings` value and composes the long-lived overlay with a fresh system-cursor effector.

**Files:**
- Modify: `internal/control/control.go` (the `Settings` struct, the `newEnlarger` field and parameter, `rebuild`, and a new setter)
- Test: `internal/control/control_test.go` (append; create the file if it does not exist)

**Interfaces:**
- Consumes: `cursor.Multi` from Task 2.
- Produces: `control.Settings` gains `Overlay string`; `control.New` takes `newEnlarger func(set Settings) cursor.Enlarger`; `func (c *Controller) SetOverlay(mode string)`.

- [ ] **Step 1: Write the failing test**

Append to `internal/control/control_test.go` (create it with this package clause if missing):

```go
package control

import (
	"testing"

	"github.com/JorgeLBJ/giant-cursor/internal/cursor"
)

type countingEnlarger struct{ enlarged, restored int }

func (c *countingEnlarger) Enlarge() error { c.enlarged++; return nil }
func (c *countingEnlarger) Restore() error { c.restored++; return nil }

func TestSetOverlayPersistsAndRebuilds(t *testing.T) {
	builds := 0
	var lastSeen Settings
	newEnlarger := func(set Settings) cursor.Enlarger {
		builds++
		lastSeen = set
		return &countingEnlarger{}
	}

	var saved Settings
	c := New(
		Settings{Scale: 4, Sensitivity: "medium", HoldMillis: 1000, Style: "crisp", Overlay: "off"},
		newEnlarger,
		func(s Settings) { saved = s },
	)
	if builds != 1 {
		t.Fatalf("constructor built %d enlargers, want 1", builds)
	}

	c.SetOverlay("halo")

	if got := c.Get().Overlay; got != "halo" {
		t.Errorf("Get().Overlay = %q, want halo", got)
	}
	if saved.Overlay != "halo" {
		t.Errorf("persisted Overlay = %q, want halo", saved.Overlay)
	}
	if lastSeen.Overlay != "halo" {
		t.Errorf("the rebuilt enlarger saw Overlay = %q, want halo", lastSeen.Overlay)
	}
	if builds != 2 {
		t.Errorf("built %d enlargers, want a rebuild after the change", builds)
	}
}

func TestSetOverlayRestoresBeforeSwitching(t *testing.T) {
	first := &countingEnlarger{}
	handed := false
	c := New(
		Settings{Scale: 4, Sensitivity: "medium", HoldMillis: 1000, Style: "crisp", Overlay: "off"},
		func(set Settings) cursor.Enlarger {
			if !handed {
				handed = true
				return first
			}
			return &countingEnlarger{}
		},
		nil,
	)

	c.SetOverlay("pointer")

	// Switching must never leave a stuck enlarged cursor behind.
	if first.restored == 0 {
		t.Error("the outgoing enlarger must be restored before the switch")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/control/ -v`
Expected: FAIL, compile errors on the `Overlay` field and on `newEnlarger`'s signature.

- [ ] **Step 3: Write the implementation**

In `internal/control/control.go`, make these four edits.

Add the field to `Settings`:

```go
// Settings are the values the user can change at runtime.
type Settings struct {
	Scale       int
	Sensitivity string
	HoldMillis  int64
	Style       string
	Overlay     string // config.OverlayMode value: off | halo | pointer
}
```

Change the field on `Controller`:

```go
	newEnlarger func(set Settings) cursor.Enlarger
```

Change `New`'s parameter and `rebuild`'s call:

```go
// New builds a Controller. newEnlarger creates an enlarger for a settings
// value; onChange (optional) is called after every change so callers can
// persist. newEnlarger receives the whole Settings so it can compose effectors
// that depend on more than one field.
func New(set Settings, newEnlarger func(set Settings) cursor.Enlarger, onChange func(Settings)) *Controller {
	c := &Controller{set: set, newEnlarger: newEnlarger, onChange: onChange}
	c.rebuild()
	return c
}

// rebuild recreates the enlarger, detector, and app from the current settings.
// Callers must hold c.mu (or be in the constructor).
func (c *Controller) rebuild() {
	c.cur = c.newEnlarger(c.set)
	det := shake.New(config.ShakeConfig(config.Sensitivity(c.set.Sensitivity), c.set.HoldMillis))
	c.app = app.New(det, c.cur)
}
```

Add the setter next to `SetStyle`:

```go
// SetOverlay changes the game overlay mode (off / halo / pointer).
func (c *Controller) SetOverlay(mode string) { c.apply(func() { c.set.Overlay = mode }) }
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/control/ -v` then `go vet ./internal/control/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/control/control.go internal/control/control_test.go
git commit -m "feat: carry the overlay mode through the controller"
```

---

### Task 8: The tray menu entry and its translations

**Files:**
- Modify: `internal/i18n/i18n.go` (the `Strings` struct and both language values)
- Modify: `internal/i18n/i18n_test.go:26-32` (`TestParallelLabelLengths`)
- Modify: `internal/lifecycle/tray_windows.go` (the id const block, `TrayCallbacks`, `showMenu`, `dispatch`)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `i18n.Strings` gains `MenuOverlay string` and `Overlays []string`; `lifecycle.TrayCallbacks` gains `Overlays []string`, `CurrentOverlay func() string`, `OnOverlay func(value string)`.

- [ ] **Step 1: Write the failing test**

Replace `TestParallelLabelLengths` in `internal/i18n/i18n_test.go` and add a beta-marker test:

```go
func TestParallelLabelLengths(t *testing.T) {
	for _, s := range []Strings{en, es} {
		if len(s.Styles) != 2 || len(s.Sensitivities) != 3 || len(s.Holds) != 3 || len(s.Langs) != 2 || len(s.Overlays) != 3 {
			t.Fatalf("label slice lengths differ from the expected value lists: %+v", s)
		}
	}
}

func TestOverlayMenuIsMarkedBeta(t *testing.T) {
	for _, s := range []Strings{en, es} {
		if !strings.Contains(strings.ToLower(s.MenuOverlay), "beta") {
			t.Errorf("the overlay menu label must carry the beta marker, got %q", s.MenuOverlay)
		}
	}
}
```

Add `"strings"` to that file's imports.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/i18n/ -v`
Expected: FAIL, compile error `s.Overlays undefined` and `s.MenuOverlay undefined`.

- [ ] **Step 3: Write the implementation**

In `internal/i18n/i18n.go`, add the two fields to `Strings`:

```go
type Strings struct {
	MenuCursor      string
	MenuSize        string
	MenuSensitivity string
	MenuHold        string
	MenuLanguage    string
	MenuOverlay     string
	Autostart       string
	Quit            string

	Styles        []string // parallel to the style value list
	Sensitivities []string // parallel to low/medium/high
	Holds         []string // parallel to the hold value list
	Langs         []string // parallel to the lang value list
	Overlays      []string // parallel to off/halo/pointer
}
```

Add the values to `en`:

```go
	MenuOverlay:   "Game overlay (beta)",
	Overlays:      []string{"Off", "Halo ring", "Large pointer"},
```

And to `es`:

```go
	MenuOverlay:   "Overlay en juegos (beta)",
	Overlays:      []string{"Desactivado", "Anillo", "Puntero grande"},
```

In `internal/lifecycle/tray_windows.go`, add the id base to the const block. It must sit above 700: the style case claims 500-599 and the language case claims 600-699, so anything lower would be swallowed.

```go
	idStyleBase       = 500 // idStyleBase + index
	idLangBase        = 600 // idLangBase + index
	idOverlayBase     = 700 // idOverlayBase + index
	idQuit            = 900
```

Add the three fields to `TrayCallbacks`, each next to its existing neighbours:

```go
	Overlays      []string // e.g. ["off","halo","pointer"]
```
```go
	CurrentOverlay     func() string
```
```go
	OnOverlay         func(value string)
```

In `showMenu`, add the submenu immediately after the language submenu block and before the autostart item:

```go
	overlayMenu := createSub()
	for i, v := range trayCB.Overlays {
		appendMenu(overlayMenu, checkFlag(v == trayCB.CurrentOverlay()), uintptr(idOverlayBase+i), labelAt(s.Overlays, i, v))
	}
	appendMenu(menu, mfString|mfPopup, overlayMenu, s.MenuOverlay)
```

In `dispatch`, add the case immediately after the language case:

```go
	case id >= idOverlayBase && id < idOverlayBase+100:
		i := id - idOverlayBase
		if i >= 0 && i < len(trayCB.Overlays) {
			trayCB.OnOverlay(trayCB.Overlays[i])
		}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./... -v` then `go vet ./...`
Expected: PASS. The app does not compile yet if the tray callbacks are required, but they are plain struct fields, so a nil `CurrentOverlay` would panic only when the menu opens. Task 9 fills them in.

- [ ] **Step 5: Commit**

```bash
git add internal/i18n/ internal/lifecycle/tray_windows.go
git commit -m "feat: add the game overlay tray menu in both languages"
```

---

### Task 9: Wire the overlay into the app

**Files:**
- Modify: `cmd/giant-cursor/main.go:33-42` (the `settings` struct), `:127-136` (flags), `:165-178` (flag merge), `:194-216` (enlarger construction), `:222-258` (tray callbacks)
- Modify: `README.md` (a section documenting the feature and its limits)

**Interfaces:**
- Consumes: everything from Tasks 1 through 8.
- Produces: a working feature.

- [ ] **Step 1: Add the persisted setting and the flag**

In `cmd/giant-cursor/main.go`, add the field to `settings`:

```go
type settings struct {
	Scale       int    `json:"scale"`
	Sensitivity string `json:"sensitivity"`
	HoldMillis  int64  `json:"hold_ms"`
	Style       string `json:"style"`
	Lang        string `json:"lang"`
	Overlay     string `json:"overlay"`
	BaseSize    int    `json:"base_cursor_size"` // user's normal cursor size (px)
}
```

Add the flag next to the others:

```go
	overlayMode := flag.String("overlay", "off", "game overlay: off|halo|pointer (beta, needs windowed mode)")
```

Add the default to the `loadSettings` call and the flag override case:

```go
	s := loadSettings(settings{Scale: *scale, Sensitivity: *sens, HoldMillis: *hold, Style: string(cursor.StyleCrisp), Lang: defaultLang(), Overlay: string(config.OverlayOff)})
```
```go
		case "overlay":
			s.Overlay = *overlayMode
```

Add `"github.com/JorgeLBJ/giant-cursor/internal/config"` and `"github.com/JorgeLBJ/giant-cursor/internal/overlay"` to the imports.

- [ ] **Step 2: Compose the overlay with the system cursor effector**

Replace the `newEnlarger` closure and the `control.New` call:

```go
	normal := s.BaseSize

	// One overlay for the whole process. The controller rebuilds the effector
	// on every settings change, so creating it inside newEnlarger would leak a
	// window per menu click; the mode and scale are pushed into it instead.
	ov := overlay.New()

	newEnlarger := func(set control.Settings) cursor.Enlarger {
		ov.SetMode(config.ParseOverlayMode(set.Overlay))
		ov.SetScale(cursor.EnlargedSize(normal, set.Scale))
		return cursor.Multi{
			cursor.NewWin32(set.Scale, normal, cursor.Style(set.Style)),
			ov,
		}
	}

	ctrl := control.New(
		control.Settings{Scale: s.Scale, Sensitivity: s.Sensitivity, HoldMillis: s.HoldMillis, Style: s.Style, Overlay: s.Overlay},
		newEnlarger,
		func(ns control.Settings) {
			s.Scale, s.Sensitivity, s.HoldMillis, s.Style, s.Overlay = ns.Scale, ns.Sensitivity, ns.HoldMillis, ns.Style, ns.Overlay
			_ = saveSettings(s)
		},
	)
```

- [ ] **Step 3: Fill in the tray callbacks**

Add the three entries to the `lifecycle.TrayCallbacks` literal, each beside its neighbours:

```go
		Overlays:      []string{string(config.OverlayOff), string(config.OverlayHalo), string(config.OverlayPointer)},
```
```go
		CurrentOverlay:     func() string { return ctrl.Get().Overlay },
```
```go
		OnOverlay:          ctrl.SetOverlay,
```

- [ ] **Step 4: Verify the whole thing builds, vets and tests clean**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all green.

Then run it and check by hand:

```bash
go run ./cmd/giant-cursor -overlay halo
```

Verify: the tray menu shows "Game overlay (beta)" with three options and the checkmark on the current one; shaking the mouse shows a ring that follows the pointer; the ring never swallows a click; switching to Off makes it disappear immediately; opening the menu several times and switching modes does not slow the app down, which would signal a leaked window.

- [ ] **Step 5: Document the feature and its limits in the README**

Add this section to `README.md`, after the existing usage material:

```markdown
### Game overlay (beta)

Games such as World of Warcraft ship their own cursor artwork, so enlarging the
Windows cursor does not reach them. The overlay draws a separate always-on-top
marker that follows the pointer instead.

Pick a mode from the tray menu, or start with `-overlay halo`:

- **Halo ring** draws a ring around the pointer. Recommended.
- **Large pointer** draws a big arrow. You will see two cursors, yours and the
  game's, because the game keeps drawing its own.

Two things to know:

- **Play in windowed or borderless mode.** Exclusive fullscreen bypasses the
  desktop compositor, so nothing drawn on top of it is visible.
- **The overlay never touches the game.** It is an ordinary window: nothing is
  injected into the game process, nothing is hooked, and no game memory is read.
```

- [ ] **Step 6: Commit**

```bash
git add cmd/giant-cursor/main.go README.md
git commit -m "feat: wire the beta game overlay into the app"
```

---

## Self-Review

**Spec coverage:**

| Spec section | Task |
|---|---|
| `cursor.Multi` composition | 2 |
| `Tracker` optional port extension | 2, 3 |
| Paint once, move often | 6 |
| Window extended styles, click-through | 6 |
| Painter interface | 5 |
| Halo geometry (2.5x, 12%, dark edges) | 5 |
| Pointer painter reusing artwork | 1, 5 |
| `ArrowImage` helper | 1 |
| `OverlayMode` enum, unknown falls back to off | 4 |
| Default off, no window when off | 4, 6 |
| Mode switching at runtime | 6, 7 |
| Tray submenu with the beta marker | 8 |
| Own thread and message loop | 6 |
| Error handling, fall back to off for the session | 6 |
| Testing plan | 1, 2, 3, 4, 5, 7 |
| Exclusive fullscreen documented | 9 |
| No injection, documented | 9 |

**Deviations from the spec, both deliberate:**

1. The spec wrote `ArrowImage(size int) image.Image`. The pointer painter needs the hotspot too, otherwise the arrow tip cannot sit on the pointer, so it returns `(*image.RGBA, int, int)`.
2. The spec did not account for `control.Controller.rebuild()` recreating the effector on every settings change, which would have leaked one Win32 window per menu click. The overlay is therefore constructed once in `main` and reconfigured, and `newEnlarger` takes the whole `Settings` value.

**Placeholder scan:** none found.

**Type consistency:** `Painter.Draw` returns `(*image.RGBA, int, int)` in Tasks 5 and 6. `cursor.ArrowImage` returns `(*image.RGBA, int, int)` in Tasks 1 and 5. `newEnlarger` is `func(set Settings) cursor.Enlarger` in Tasks 7 and 9. `Track(x, y int)` is consistent across Tasks 2, 3 and 6.
