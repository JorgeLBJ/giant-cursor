package config

// OverlayMode selects what the game overlay draws, if anything. Games that ship
// their own cursor art ignore SetSystemCursor, so the overlay is the only way to
// make the pointer findable inside them.
type OverlayMode string

const (
	// OverlayOff draws nothing and creates no overlay window at all.
	OverlayOff OverlayMode = "off"
	// OverlayHalo draws a ring around the pointer. It annotates rather than
	// replaces, so it does not compete with the cursor the game draws itself.
	OverlayHalo OverlayMode = "halo"
	// OverlayPointer draws an enlarged arrow at the pointer. The game keeps
	// drawing its own, so the user sees two cursors: expected in this mode.
	OverlayPointer OverlayMode = "pointer"
)

// ParseOverlayMode maps a stored value to a mode. Anything unknown or empty
// falls back to OverlayOff, so a config file written before this feature
// existed keeps the overlay switched off.
func ParseOverlayMode(v string) OverlayMode {
	switch OverlayMode(v) {
	case OverlayHalo:
		return OverlayHalo
	case OverlayPointer:
		return OverlayPointer
	default:
		return OverlayOff
	}
}
