package cursor

import "testing"

func TestArtworkBitmaps(t *testing.T) {
	const size = 128
	for name, art := range map[string]*artwork{"arrow": arrowArt, "hand": handArt} {
		buf, hotX, hotY := art.bitmap(size)

		if len(buf) != size*size*4 {
			t.Fatalf("%s: buffer len = %d, want %d", name, len(buf), size*size*4)
		}
		// The artwork must actually load and draw something.
		opaque := 0
		for i := 3; i < len(buf); i += 4 {
			if buf[i] > 0 {
				opaque++
			}
		}
		if opaque == 0 {
			t.Fatalf("%s: bitmap is fully transparent (artwork failed to load)", name)
		}
		// The hotspot must land inside the drawn artwork.
		if hotX < 0 || hotX >= size || hotY < 0 || hotY >= size {
			t.Errorf("%s: hotspot (%d,%d) is outside a %d box", name, hotX, hotY, size)
		}
		if a := buf[(hotY*size+hotX)*4+3]; a == 0 {
			t.Errorf("%s: hotspot (%d,%d) sits on a transparent pixel", name, hotX, hotY)
		}
		// The bottom-right corner is outside the artwork.
		if a := buf[(size*size-1)*4+3]; a != 0 {
			t.Errorf("%s: expected transparent corner, got alpha=%d", name, a)
		}
		// Premultiplied invariant: no colour channel may exceed alpha.
		for i := 0; i < len(buf); i += 4 {
			if buf[i+2] > buf[i+3] {
				t.Fatalf("%s: premultiplied violation at %d: r=%d a=%d", name, i, buf[i+2], buf[i+3])
			}
		}
	}
}

func TestArtworkScalesWithSize(t *testing.T) {
	countOpaque := func(size int) int {
		buf, _, _ := arrowArt.bitmap(size)
		n := 0
		for i := 3; i < len(buf); i += 4 {
			if buf[i] > 0 {
				n++
			}
		}
		return n
	}
	if countOpaque(256) <= countOpaque(64) {
		t.Error("a larger cursor should cover more pixels")
	}
}

func TestArrowImage(t *testing.T) {
	const size = 128
	img, hotX, hotY := ArrowImage(size)

	if b := img.Bounds(); b.Dx() != size || b.Dy() != size {
		t.Fatalf("bounds = %dx%d, want %dx%d", b.Dx(), b.Dy(), size, size)
	}
	if _, _, _, a := img.At(hotX, hotY).RGBA(); a == 0 {
		t.Errorf("hotspot (%d,%d) sits on a transparent pixel", hotX, hotY)
	}
	if _, _, _, a := img.At(size-1, size-1).RGBA(); a != 0 {
		t.Errorf("bottom-right corner should be outside the artwork, got alpha=%d", a)
	}
}

func TestArrowImageMatchesBitmapHotspot(t *testing.T) {
	const size = 96
	_, wantX, wantY := arrowArt.bitmap(size)
	_, gotX, gotY := ArrowImage(size)

	if gotX != wantX || gotY != wantY {
		t.Errorf("ArrowImage hotspot = (%d,%d), bitmap hotspot = (%d,%d)", gotX, gotY, wantX, wantY)
	}
}
