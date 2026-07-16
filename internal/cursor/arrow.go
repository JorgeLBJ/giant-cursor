package cursor

import "math"

// arrowPolygon is the classic pointer silhouette (Windows / X11 left_ptr) in a
// unit box (y grows down), tip at (0,0). These are the real cursor's vertices
// normalized from its 16-unit grid, not eyeballed values.
var arrowPolygon = [][2]float64{
	{0.0000, 0.0000}, // tip
	{0.0000, 0.8750}, // left edge / left barb bottom
	{0.1875, 0.6875}, // inner notch, left of the tail
	{0.3125, 1.0000}, // tail (leg) tip
	{0.4375, 0.9375}, // tail bottom-right
	{0.3125, 0.6250}, // inner notch, right of the tail
	{0.5625, 0.6250}, // right shoulder
}

// rasterizeArrow renders a crisp, anti-aliased white arrow with a thin black
// outline that straddles the silhouette (so thin parts keep their white core,
// like the real cursor). It returns a top-down 32bpp premultiplied BGRA buffer
// (size*size*4 bytes). The hotspot is the tip (0,0).
// arrowFill is the fraction of the bitmap the arrow occupies. The real Windows
// arrow fills only ~62% of its cursor box (the rest is padding), so matching it
// keeps our crisp arrow the same visual size as the upscaled system cursor.
const arrowFill = 0.62

func rasterizeArrow(size int) []byte {
	if size < 1 {
		size = 1
	}
	scale := float64(size-1) * arrowFill
	poly := make([][2]float64, len(arrowPolygon))
	for i, p := range arrowPolygon {
		poly[i] = [2]float64{p[0] * scale, p[1] * scale}
	}

	// Outline half-width: the black rim extends this far on each side of the
	// silhouette edge, giving a clean thin outline that never eats the interior.
	half := float64(size) * 0.018
	if half < 1 {
		half = 1
	}

	const ss = 4 // supersampling per axis for anti-aliasing
	total := float64(ss * ss)
	buf := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			white, ink := 0, 0
			for jy := 0; jy < ss; jy++ {
				for jx := 0; jx < ss; jx++ {
					px := float64(x) + (float64(jx)+0.5)/ss
					py := float64(y) + (float64(jy)+0.5)/ss
					d := distToPolygon(px, py, poly)
					switch {
					case d <= half: // straddling outline (inside or outside)
						ink++
					case pointInPolygon(px, py, poly): // interior beyond the rim
						white++
					}
				}
			}
			cov := white + ink
			if cov == 0 {
				continue // transparent (buffer already zero)
			}
			o := (y*size + x) * 4
			w := byte(float64(white) / total * 255) // premultiplied white channel
			buf[o+0] = w                             // B
			buf[o+1] = w                             // G
			buf[o+2] = w                             // R
			buf[o+3] = byte(float64(cov) / total * 255) // A
		}
	}
	return buf
}

func pointInPolygon(x, y float64, poly [][2]float64) bool {
	in := false
	n := len(poly)
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := poly[i][0], poly[i][1]
		xj, yj := poly[j][0], poly[j][1]
		if (yi > y) != (yj > y) {
			xc := (xj-xi)*(y-yi)/(yj-yi) + xi
			if x < xc {
				in = !in
			}
		}
		j = i
	}
	return in
}

func distToPolygon(x, y float64, poly [][2]float64) float64 {
	best := math.Inf(1)
	n := len(poly)
	j := n - 1
	for i := 0; i < n; i++ {
		if d := distToSegment(x, y, poly[j][0], poly[j][1], poly[i][0], poly[i][1]); d < best {
			best = d
		}
		j = i
	}
	return best
}

func distToSegment(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	if dx == 0 && dy == 0 {
		return math.Hypot(px-ax, py-ay)
	}
	t := ((px-ax)*dx + (py-ay)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return math.Hypot(px-(ax+t*dx), py-(ay+t*dy))
}
