//go:build !windows

package overlay

import "github.com/JorgeLBJ/giant-cursor/internal/config"

// Overlay is a no-op outside Windows. The app only ships for Windows; this
// stub exists so the package still builds and its painters stay testable
// on any developer machine.
type Overlay struct{}

// New returns an inert overlay.
func New() *Overlay { return &Overlay{} }

// Enlarge does nothing.
func (o *Overlay) Enlarge() error { return nil }

// Restore does nothing.
func (o *Overlay) Restore() error { return nil }

// Track does nothing.
func (o *Overlay) Track(x, y int) {}

// SetMode does nothing.
func (o *Overlay) SetMode(mode config.OverlayMode) {}

// SetScale does nothing.
func (o *Overlay) SetScale(cursorSize int) {}
