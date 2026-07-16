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
