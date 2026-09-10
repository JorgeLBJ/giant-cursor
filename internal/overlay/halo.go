package overlay

import (
	"image"
	"image/color"
	"math"
)

// Starting geometry from the design document, to be tuned against real play.
// haloOuter is the ring diameter as a multiple of the enlarged cursor size;
// haloThickness is the ring thickness as a fraction of that diameter.
const (
	haloOuter     = 2.5
	haloThickness = 0.12
)

// Halo draws a ring centred on the pointer. It annotates instead of replacing,
// so it never competes with the cursor the game draws itself, and a thick soft
// shape hides the few pixels of lag between our window and the game's frame.
type Halo struct{}

// Size returns the ring's bounding box edge in pixels.
func (Halo) Size(cursorSize int) int {
	s := int(float64(cursorSize) * haloOuter)
	if s < 1 {
		s = 1
	}
	return s
}

// Draw renders the ring, anchored at its centre.
func (h Halo) Draw(cursorSize int) (*image.RGBA, int, int) {
	size := h.Size(cursorSize)
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	centre := float64(size) / 2
	outer := centre
	inner := outer - float64(size)*haloThickness
	if inner < 0 {
		inner = 0
	}
	// A dark border on both edges of the ring keeps it readable against both
	// bright snow and a dark dungeon.
	border := (outer - inner) / 3

	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - centre
			dy := float64(y) + 0.5 - centre
			d := math.Sqrt(dx*dx + dy*dy)
			switch {
			case d > outer || d < inner:
				continue
			case d > outer-border || d < inner+border:
				img.SetRGBA(x, y, black)
			default:
				img.SetRGBA(x, y, white)
			}
		}
	}
	return img, size / 2, size / 2
}
