package shake

import "testing"

func TestNewDetectorStartsNormal(t *testing.T) {
	d := New(Config{})
	if d.State() != StateNormal {
		t.Fatalf("want NORMAL, got %v", d.State())
	}
}

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
	return Config{WindowMillis: 500, MinReversals: 4, NoiseFloor: 3, HoldMillis: 1000}
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

func TestHoldExpiresReturnsToNormal(t *testing.T) {
	d := New(shakeCfg())
	feed(d, []int32{0, 30, 0, 30, 0, 30, 0}, []int32{0, 0, 0, 0, 0, 0, 0}, 20)
	if d.State() != StateBig {
		t.Fatalf("precondition failed: want BIG, got %v", d.State())
	}
	// Well past HoldMillis (1000ms) with no further shaking -> NORMAL.
	st := d.Update(Sample{Pos: Point{X: 0}, Millis: 10000})
	if st != StateNormal {
		t.Fatalf("want NORMAL after hold, got %v", st)
	}
}

func TestSmoothMovementShrinksAfterHold(t *testing.T) {
	d := New(shakeCfg())
	feed(d, []int32{0, 30, 0, 30, 0, 30, 0}, []int32{0, 0, 0, 0, 0, 0, 0}, 20)
	if d.State() != StateBig {
		t.Fatalf("precondition failed: want BIG, got %v", d.State())
	}
	// Move smoothly (no reversals): it must shrink after the hold EVEN while
	// the mouse keeps moving. This is the core fix: movement no longer holds it.
	var ms int64 = 140
	var x int32 = 100
	st := d.State()
	for i := 0; i < 60; i++ {
		x += 10
		ms += 50
		st = d.Update(Sample{Pos: Point{X: x}, Millis: ms})
	}
	if st != StateNormal {
		t.Fatalf("want NORMAL after hold while moving smoothly, got %v", st)
	}
}

func TestReshakeKeepsBig(t *testing.T) {
	d := New(shakeCfg())
	feed(d, []int32{0, 30, 0, 30, 0, 30, 0}, []int32{0, 0, 0, 0, 0, 0, 0}, 20)
	if d.State() != StateBig {
		t.Fatalf("precondition failed: want BIG, got %v", d.State())
	}
	// Keep shaking well past the hold; continued shakes must re-arm and keep BIG.
	var ms int64 = 140
	st := d.State()
	for i := 0; i < 200; i++ {
		var x int32
		if i%2 == 1 {
			x = 30
		}
		ms += 20
		st = d.Update(Sample{Pos: Point{X: x}, Millis: ms})
	}
	if st != StateBig {
		t.Fatalf("want BIG while still shaking, got %v", st)
	}
}
