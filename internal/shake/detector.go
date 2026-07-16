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

// Config tunes shake and hold behaviour.
type Config struct {
	WindowMillis int64 // sliding window for counting reversals
	MinReversals int   // reversals within window to trigger BIG
	NoiseFloor   int32 // per-step axis delta below this (px) is ignored
	HoldMillis   int64 // time to stay BIG after the last detected shake
}

// Detector is a stateful shake/hold detector. Not safe for concurrent use;
// call Update from a single goroutine.
type Detector struct {
	cfg             Config
	samples         []Sample
	state           State
	lastShakeMillis int64
}

// New returns a Detector in the NORMAL state.
func New(cfg Config) *Detector {
	return &Detector{cfg: cfg, state: StateNormal}
}

// State returns the current state.
func (d *Detector) State() State { return d.state }

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// Update feeds one sample and returns the resulting state.
//
// A shake (enough direction reversals within the window) enlarges the cursor
// and arms a hold timer. The cursor stays BIG until HoldMillis pass with no
// further shake — plain smooth movement does NOT keep it big. Shaking again
// re-arms the hold.
func (d *Detector) Update(s Sample) State {
	d.samples = append(d.samples, s)
	cutoff := s.Millis - d.cfg.WindowMillis
	drop := 0
	for drop < len(d.samples) && d.samples[drop].Millis < cutoff {
		drop++
	}
	if drop > 0 {
		d.samples = d.samples[drop:]
	}

	shaking := d.reversals() >= d.cfg.MinReversals

	switch d.state {
	case StateNormal:
		if shaking {
			d.state = StateBig
			d.lastShakeMillis = s.Millis
			d.samples = d.samples[:0] // require a fresh burst to re-arm
		}
	case StateBig:
		switch {
		case shaking:
			d.lastShakeMillis = s.Millis
			d.samples = d.samples[:0]
		case s.Millis-d.lastShakeMillis >= d.cfg.HoldMillis:
			d.state = StateNormal
			d.samples = d.samples[:0]
		}
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
