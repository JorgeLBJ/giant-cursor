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
