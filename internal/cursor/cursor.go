// Package cursor defines the port for enlarging and restoring the OS cursor.
package cursor

// MaxBaseSize is the largest cursor base size Windows supports (accessibility
// size 15). DefaultBaseSize is the standard size (accessibility size 1).
const (
	DefaultBaseSize = 32
	MaxBaseSize     = 256
)

// Style selects how the cursor is enlarged.
type Style string

const (
	// StyleCrisp draws a custom high-resolution arrow (sharp at any size).
	StyleCrisp Style = "crisp"
	// StyleSystem upscales the user's actual system cursors (softer, but keeps
	// every cursor shape exactly as the user has it).
	StyleSystem Style = "system"
	// StyleNative asks Windows to render its own cursors at a large base size
	// (crisp, identical to the user's real cursors) where the OS supports it.
	StyleNative Style = "native"
)

// Enlarger swaps the system cursors to enlarged copies and restores them.
type Enlarger interface {
	Enlarge() error
	Restore() error
}

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

// EnlargedSize returns the enlarged cursor base size in pixels for a normal
// base size and scale factor, clamped to the Windows range.
func EnlargedSize(normal, scale int) int {
	if normal < 1 {
		normal = DefaultBaseSize
	}
	if scale < 1 {
		scale = 1
	}
	size := normal * scale
	if size > MaxBaseSize {
		size = MaxBaseSize
	}
	return size
}
