// Package i18n holds the tray menu translations. Label slices are parallel to
// the value lists the tray iterates (styles, sensitivities, holds, langs).
package i18n

// Strings are all the user-visible tray labels for one language.
type Strings struct {
	MenuCursor      string
	MenuSize        string
	MenuSensitivity string
	MenuHold        string
	MenuLanguage    string
	MenuOverlay     string
	Autostart       string
	Quit            string

	Styles        []string // parallel to the style value list
	Sensitivities []string // parallel to low/medium/high
	Holds         []string // parallel to the hold value list
	Langs         []string // parallel to the lang value list
	Overlays      []string // parallel to off/halo/pointer
}

// For returns the Strings for the given language code, defaulting to English.
func For(lang string) Strings {
	if lang == "es" {
		return es
	}
	return en
}

var en = Strings{
	MenuCursor:      "Cursor",
	MenuSize:        "Size",
	MenuSensitivity: "Sensitivity",
	MenuHold:        "Enlarged for",
	MenuLanguage:    "Language",
	MenuOverlay:     "Game overlay (beta)",
	Autostart:       "Start with Windows",
	Quit:            "Quit Giant Cursor",
	Styles:          []string{"Crisp arrow", "System (zoom)"},
	Sensitivities:   []string{"Low", "Medium", "High"},
	Holds:           []string{"Short (0.7s)", "Normal (1s)", "Long (1.5s)"},
	Langs:           []string{"English", "Español"},
	Overlays:        []string{"Off", "Halo ring", "Large pointer"},
}

var es = Strings{
	MenuCursor:      "Cursor",
	MenuSize:        "Tamaño",
	MenuSensitivity: "Sensibilidad",
	MenuHold:        "Grande por",
	MenuLanguage:    "Idioma",
	MenuOverlay:     "Overlay en juegos (beta)",
	Autostart:       "Iniciar con Windows",
	Quit:            "Salir de Giant Cursor",
	Styles:          []string{"Flecha nítida", "Sistema (zoom)"},
	Sensitivities:   []string{"Baja", "Media", "Alta"},
	Holds:           []string{"Corto (0.7s)", "Normal (1s)", "Largo (1.5s)"},
	Langs:           []string{"English", "Español"},
	Overlays:        []string{"Desactivado", "Anillo", "Puntero grande"},
}
