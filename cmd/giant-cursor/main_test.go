package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeConfig points configPath at a temporary directory and puts body in it.
func writeConfig(t *testing.T, body string) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	if err := os.MkdirAll(filepath.Join(dir, "giant-cursor"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "giant-cursor", "config.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestLoadSettingsDefaultsOverlayWhenTheKeyIsAbsent(t *testing.T) {
	// A config written before the overlay existed. The value must become the
	// default, not the empty string: the tray marks the current option by
	// comparing it against the option values, so an empty value leaves every
	// option unmarked and the menu shows no selection at all.
	writeConfig(t, `{"scale":4,"sensitivity":"medium","hold_ms":1500,"style":"crisp","lang":"es","base_cursor_size":32}`)

	s := loadSettings(settings{Scale: 4, Sensitivity: "medium", HoldMillis: 1000, Style: "crisp", Lang: "en", Overlay: "off"})

	if s.Overlay != "off" {
		t.Errorf("Overlay = %q, want off", s.Overlay)
	}
}

func TestLoadSettingsKeepsAStoredOverlay(t *testing.T) {
	writeConfig(t, `{"scale":4,"sensitivity":"medium","hold_ms":1500,"style":"crisp","lang":"es","overlay":"halo","base_cursor_size":32}`)

	s := loadSettings(settings{Scale: 4, Sensitivity: "medium", HoldMillis: 1000, Style: "crisp", Lang: "en", Overlay: "off"})

	if s.Overlay != "halo" {
		t.Errorf("Overlay = %q, want halo", s.Overlay)
	}
}
