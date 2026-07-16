// Package config maps human-friendly sensitivity levels to a shake.Config.
package config

import "giant-cursor/internal/shake"

// Sensitivity names how vigorous a shake must be to trigger enlargement.
type Sensitivity string

const (
	Low    Sensitivity = "low"
	Medium Sensitivity = "medium"
	High   Sensitivity = "high"
)

// ShakeConfig builds a shake.Config for the given sensitivity and hold time.
// Unknown values fall back to Medium. Starting thresholds; tune with real use.
func ShakeConfig(s Sensitivity, holdMillis int64) shake.Config {
	cfg := shake.Config{
		WindowMillis: 400,
		MinReversals: 4,
		NoiseFloor:   4,
		HoldMillis:   holdMillis,
	}
	switch s {
	case High:
		cfg.MinReversals = 3
		cfg.WindowMillis = 500
		cfg.NoiseFloor = 2
	case Low:
		cfg.MinReversals = 6
		cfg.WindowMillis = 350
		cfg.NoiseFloor = 6
	default: // Medium and any unknown value
	}
	return cfg
}
