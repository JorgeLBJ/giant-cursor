// Package app wires the shake detector to a cursor Enlarger, calling Enlarge
// on the NORMAL->BIG transition and Restore on BIG->NORMAL.
package app

import (
	"giant-cursor/internal/cursor"
	"giant-cursor/internal/shake"
)

// App drives the enlarge/restore side effects from detector state changes.
type App struct {
	det   *shake.Detector
	cur   cursor.Enlarger
	state shake.State
}

// New returns an App in the NORMAL state.
func New(det *shake.Detector, cur cursor.Enlarger) *App {
	return &App{det: det, cur: cur, state: shake.StateNormal}
}

// Step feeds one sample to the detector and applies the resulting side effect.
func (a *App) Step(s shake.Sample) error {
	prev := a.state
	next := a.det.Update(s)
	a.state = next
	switch {
	case prev == shake.StateNormal && next == shake.StateBig:
		return a.cur.Enlarge()
	case prev == shake.StateBig && next == shake.StateNormal:
		return a.cur.Restore()
	}
	return nil
}
