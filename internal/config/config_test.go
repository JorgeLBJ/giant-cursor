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

func TestIdleMillisPassthrough(t *testing.T) {
	if ShakeConfig(Medium, 750).IdleMillis != 750 {
		t.Fatal("idle millis not propagated")
	}
}
