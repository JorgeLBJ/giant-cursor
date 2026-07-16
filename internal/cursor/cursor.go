// Package cursor defines the port for enlarging and restoring the OS cursor.
package cursor

// MaxBaseSize is the largest cursor base size Windows supports (accessibility
// size 15). DefaultBaseSize is the standard size (accessibility size 1).
const (
	DefaultBaseSize = 32
	MaxBaseSize     = 256
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
