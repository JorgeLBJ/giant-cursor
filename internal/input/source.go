// Package input defines the port for sampling the cursor position.
package input

import "github.com/JorgeLBJ/giant-cursor/internal/shake"

// PositionSource returns the current cursor position.
type PositionSource interface {
	Poll() (shake.Point, error)
}
