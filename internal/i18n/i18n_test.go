package i18n

import "testing"

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
		if len(s.Styles) != 2 || len(s.Sensitivities) != 3 || len(s.Holds) != 3 || len(s.Langs) != 2 {
			t.Fatalf("label slice lengths differ from the expected value lists: %+v", s)
		}
	}
}
