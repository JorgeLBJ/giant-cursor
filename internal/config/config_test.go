package config

import "testing"

func TestSensitivityOrdering(t *testing.T) {
	if ShakeConfig(High, 1000).MinReversals >= ShakeConfig(Low, 1000).MinReversals {
		t.Fatal("High sensitivity must need fewer reversals than Low")
	}
}

func TestUnknownDefaultsToMedium(t *testing.T) {
	if ShakeConfig("bogus", 1000).MinReversals != ShakeConfig(Medium, 1000).MinReversals {
		t.Fatal("unknown sensitivity should default to Medium")
	}
}

func TestHoldMillisPassthrough(t *testing.T) {
	if ShakeConfig(Medium, 750).HoldMillis != 750 {
		t.Fatal("hold millis not propagated")
	}
}
