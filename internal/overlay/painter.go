// Package overlay draws a pointer aid on top of everything else. Games that
// ship their own cursor art ignore SetSystemCursor, so a separate always-on-top
// window is the only way to make the pointer findable inside them without
// injecting into the game, which anti-cheat systems treat as an attack.
package overlay

import (
	"image"

	"github.com/JorgeLBJ/giant-cursor/internal/config"
)

// Painter renders the overlay bitmap for a given enlarged cursor size. The
// window is the expensive part, not the shape, so swapping painters is how the
// user picks a look without paying for a second implementation.
type Painter interface {
	// Size returns the window edge length in pixels for a cursor size.
	Size(cursorSize int) int
	// Draw renders the art in premultiplied RGBA and returns the point inside
	// the image that must sit exactly on the pointer.
	Draw(cursorSize int) (img *image.RGBA, anchorX, anchorY int)
}

// PainterFor returns the painter for a mode, or nil for config.OverlayOff.
// A nil painter means no window is created at all.
func PainterFor(mode config.OverlayMode) Painter {
	switch mode {
	case config.OverlayHalo:
		return Halo{}
	case config.OverlayPointer:
		return Pointer{}
	default:
		return nil
	}
}
