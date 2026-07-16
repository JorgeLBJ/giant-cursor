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
