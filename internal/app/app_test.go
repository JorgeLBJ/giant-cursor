package app

import (
	"testing"

	"github.com/JorgeLBJ/giant-cursor/internal/shake"
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
		HoldMillis: 1000,
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
