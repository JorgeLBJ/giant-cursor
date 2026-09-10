package overlay

import (
	"image"

	"github.com/JorgeLBJ/giant-cursor/internal/cursor"
)

// Pointer draws an enlarged arrow at the pointer, reusing the same crisp
// artwork the system-cursor effector uses. The game keeps drawing its own
// cursor and we cannot stop it without injecting, so the user sees two
// pointers in this mode. That is expected behaviour, not a defect.
type Pointer struct{}

// Size returns the arrow's bounding box edge in pixels.
func (Pointer) Size(cursorSize int) int {
	if cursorSize < 1 {
		return 1
	}
	return cursorSize
}

// Draw renders the arrow, anchored at its tip.
func (p Pointer) Draw(cursorSize int) (*image.RGBA, int, int) {
	return cursor.ArrowImage(p.Size(cursorSize))
}
