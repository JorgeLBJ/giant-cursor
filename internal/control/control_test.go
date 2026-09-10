package control

import (
	"testing"

	"github.com/JorgeLBJ/giant-cursor/internal/cursor"
	"github.com/JorgeLBJ/giant-cursor/internal/shake"
)

type fakeEnl struct{ enl, res int }

func (f *fakeEnl) Enlarge() error { f.enl++; return nil }
func (f *fakeEnl) Restore() error { f.res++; return nil }

func TestSetScaleRebuildsAndPersists(t *testing.T) {
	var made []int
	var persisted Settings
	factory := func(set Settings) cursor.Enlarger {
		made = append(made, set.Scale)
		return &fakeEnl{}
	}
	c := New(Settings{Scale: 4, Sensitivity: "medium", HoldMillis: 1000, Style: "crisp"}, factory, func(s Settings) {
		persisted = s
	})

	if got := c.Get().Scale; got != 4 {
		t.Fatalf("initial scale = %d, want 4", got)
	}
	c.SetScale(6)
	if got := c.Get().Scale; got != 6 {
		t.Fatalf("after SetScale, scale = %d, want 6", got)
	}
	if persisted.Scale != 6 {
		t.Fatalf("persisted scale = %d, want 6", persisted.Scale)
	}
	// Enlarger factory runs once at New (4) and again on rebuild (6).
	if len(made) != 2 || made[0] != 4 || made[1] != 6 {
		t.Fatalf("factory scales = %v, want [4 6]", made)
	}
}

func TestStepEnlargesOnShake(t *testing.T) {
	fe := &fakeEnl{}
	c := New(Settings{Scale: 4, Sensitivity: "high", HoldMillis: 1000, Style: "crisp"},
		func(Settings) cursor.Enlarger { return fe }, nil)

	var ms int64
	for _, x := range []int32{0, 30, 0, 30, 0, 30, 0} {
		_ = c.Step(shake.Sample{Pos: shake.Point{X: x}, Millis: ms})
		ms += 20
	}
	if fe.enl < 1 {
		t.Fatalf("want at least one Enlarge on shake, got %d", fe.enl)
	}
}

func TestSetOverlayPersistsAndRebuilds(t *testing.T) {
	builds := 0
	var lastSeen Settings
	newEnlarger := func(set Settings) cursor.Enlarger {
		builds++
		lastSeen = set
		return &fakeEnl{}
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
	first := &fakeEnl{}
	handed := false
	c := New(
		Settings{Scale: 4, Sensitivity: "medium", HoldMillis: 1000, Style: "crisp", Overlay: "off"},
		func(set Settings) cursor.Enlarger {
			if !handed {
				handed = true
				return first
			}
			return &fakeEnl{}
		},
		nil,
	)

	c.SetOverlay("pointer")

	// Switching must never leave a stuck enlarged cursor behind.
	if first.res == 0 {
		t.Error("the outgoing enlarger must be restored before the switch")
	}
}
