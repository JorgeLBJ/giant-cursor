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
