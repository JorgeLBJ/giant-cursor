package i18n

import (
	"strings"
	"testing"
)

func TestForDefaultsToEnglish(t *testing.T) {
	if For("").Quit != en.Quit {
		t.Fatal("empty lang should default to English")
	}
	if For("fr").Quit != en.Quit {
		t.Fatal("unknown lang should default to English")
	}
}

func TestSpanishTranslations(t *testing.T) {
	s := For("es")
	if s.Quit != "Salir de Giant Cursor" {
		t.Errorf("Quit = %q", s.Quit)
	}
	if s.MenuSize != "Tamaño" {
		t.Errorf("MenuSize = %q", s.MenuSize)
	}
}

func TestParallelLabelLengths(t *testing.T) {
	for _, s := range []Strings{en, es} {
		if len(s.Styles) != 2 || len(s.Sensitivities) != 3 || len(s.Holds) != 3 || len(s.Langs) != 2 || len(s.Overlays) != 3 {
			t.Fatalf("label slice lengths differ from the expected value lists: %+v", s)
		}
	}
}

func TestOverlayMenuIsMarkedBeta(t *testing.T) {
	for _, s := range []Strings{en, es} {
		if !strings.Contains(strings.ToLower(s.MenuOverlay), "beta") {
			t.Errorf("the overlay menu label must carry the beta marker, got %q", s.MenuOverlay)
		}
	}
}
