// Package app wires the shake detector to a cursor Enlarger, calling Enlarge
// on the NORMAL->BIG transition and Restore on BIG->NORMAL.
package app

import (
	"github.com/JorgeLBJ/giant-cursor/internal/cursor"
	"github.com/JorgeLBJ/giant-cursor/internal/shake"
)

// App drives the enlarge/restore side effects from detector state changes.
type App struct {
	det   *shake.Detector
	cur   cursor.Enlarger
	trk   cursor.Tracker // nil unless the effector follows the pointer
	state shake.State
}

// New returns an App in the NORMAL state. If the effector also implements
// cursor.Tracker it receives the pointer position on every enlarged sample.
// The assertion happens once here, never per sample.
func New(det *shake.Detector, cur cursor.Enlarger) *App {
	a := &App{det: det, cur: cur, state: shake.StateNormal}
	if t, ok := cur.(cursor.Tracker); ok {
		a.trk = t
	}
	return a
}

// Step feeds one sample to the detector and applies the resulting side effect.
func (a *App) Step(s shake.Sample) error {
	prev := a.state
	next := a.det.Update(s)
	a.state = next
	switch {
	case prev == shake.StateNormal && next == shake.StateBig:
		err := a.cur.Enlarge()
		a.track(s) // place the overlay before the user can see it
		return err
	case prev == shake.StateBig && next == shake.StateNormal:
		return a.cur.Restore()
	case next == shake.StateBig:
		a.track(s)
	}
	return nil
}

// track forwards the pointer position to a tracking effector, if there is one.
func (a *App) track(s shake.Sample) {
	if a.trk != nil {
		a.trk.Track(int(s.Pos.X), int(s.Pos.Y))
	}
}
