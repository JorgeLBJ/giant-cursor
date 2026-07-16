package cursor

import "testing"

func TestRasterizeArrowShape(t *testing.T) {
	const size = 64
	buf := rasterizeArrow(size)
	if len(buf) != size*size*4 {
		t.Fatalf("buffer len = %d, want %d", len(buf), size*size*4)
	}
	alphaAt := func(x, y int) byte { return buf[(y*size+x)*4+3] }
	redAt := func(x, y int) byte { return buf[(y*size+x)*4+2] }

	// Near the tip the arrow is opaque.
	if alphaAt(1, 2) == 0 {
		t.Errorf("expected opaque near tip, got transparent")
	}
	// A point well inside the head is white (high red) and opaque.
	if a, r := alphaAt(4, 18), redAt(4, 18); a == 0 || r < 100 {
		t.Errorf("expected white interior at (4,18): alpha=%d red=%d", a, r)
	}
	// The bottom-right corner is outside the arrow (transparent).
	if a := alphaAt(size-1, size-1); a != 0 {
		t.Errorf("expected transparent corner, got alpha=%d", a)
	}
	// Premultiplied invariant: color channel never exceeds alpha.
	for i := 0; i < len(buf); i += 4 {
		if buf[i+2] > buf[i+3] {
			t.Fatalf("premultiplied violation at %d: r=%d a=%d", i, buf[i+2], buf[i+3])
		}
	}
}
