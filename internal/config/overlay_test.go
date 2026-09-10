package config

import "testing"

func TestParseOverlayModeKnownValues(t *testing.T) {
	if ParseOverlayMode("halo") != OverlayHalo {
		t.Error("halo should parse to OverlayHalo")
	}
	if ParseOverlayMode("pointer") != OverlayPointer {
		t.Error("pointer should parse to OverlayPointer")
	}
	if ParseOverlayMode("off") != OverlayOff {
		t.Error("off should parse to OverlayOff")
	}
}

func TestParseOverlayModeDefaultsToOff(t *testing.T) {
	if ParseOverlayMode("") != OverlayOff {
		t.Error("an empty value should default to off, so a config file without the key stays off")
	}
	if ParseOverlayMode("bogus") != OverlayOff {
		t.Error("an unknown value should default to off")
	}
}
