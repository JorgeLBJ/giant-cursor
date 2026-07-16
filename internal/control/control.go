// Package control owns the live-adjustable settings and rebuilds the detector
// and cursor enlarger when the user changes them from the tray menu. It is safe
// for concurrent use: the poll loop calls Step while the tray calls the setters.
package control

import (
	"sync"

	"giant-cursor/internal/app"
	"giant-cursor/internal/config"
	"giant-cursor/internal/cursor"
	"giant-cursor/internal/shake"
)

// Settings are the values the user can change at runtime.
type Settings struct {
	Scale       int
	Sensitivity string
	HoldMillis  int64
	Style       string
}

// Controller wires Settings to a detector + enlarger and applies live changes.
type Controller struct {
	mu          sync.Mutex
	set         Settings
	newEnlarger func(scale int, style string) cursor.Enlarger
	cur         cursor.Enlarger
	app         *app.App
	onChange    func(Settings)
}

// New builds a Controller. newEnlarger creates an enlarger for a given scale and
// style; onChange (optional) is called after every change so callers can persist.
func New(set Settings, newEnlarger func(scale int, style string) cursor.Enlarger, onChange func(Settings)) *Controller {
	c := &Controller{set: set, newEnlarger: newEnlarger, onChange: onChange}
	c.rebuild()
	return c
}

// rebuild recreates the enlarger, detector, and app from the current settings.
// Callers must hold c.mu (or be in the constructor).
func (c *Controller) rebuild() {
	c.cur = c.newEnlarger(c.set.Scale, c.set.Style)
	det := shake.New(config.ShakeConfig(config.Sensitivity(c.set.Sensitivity), c.set.HoldMillis))
	c.app = app.New(det, c.cur)
}

// Step feeds one sample through the current app.
func (c *Controller) Step(s shake.Sample) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.app.Step(s)
}

// apply normalizes the cursor, mutates settings, rebuilds, and notifies.
func (c *Controller) apply(mut func()) {
	c.mu.Lock()
	_ = c.cur.Restore() // avoid a stuck enlarged cursor while switching
	mut()
	c.rebuild()
	set := c.set
	onChange := c.onChange
	c.mu.Unlock()
	if onChange != nil {
		onChange(set)
	}
}

// SetScale changes the enlargement factor.
func (c *Controller) SetScale(scale int) { c.apply(func() { c.set.Scale = scale }) }

// SetSensitivity changes the shake sensitivity.
func (c *Controller) SetSensitivity(s string) { c.apply(func() { c.set.Sensitivity = s }) }

// SetHold changes how long the cursor stays enlarged after the last shake.
func (c *Controller) SetHold(ms int64) { c.apply(func() { c.set.HoldMillis = ms }) }

// SetStyle changes the enlargement style (crisp / system / native).
func (c *Controller) SetStyle(style string) { c.apply(func() { c.set.Style = style }) }

// Get returns a copy of the current settings.
func (c *Controller) Get() Settings {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.set
}

// Restore returns the cursor to its normal size.
func (c *Controller) Restore() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cur.Restore()
}
