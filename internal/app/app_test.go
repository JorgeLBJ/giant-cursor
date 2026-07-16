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
