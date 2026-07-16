// Command genarrow turns the cursor artwork in assets/cursor-raw/*.png (white
// and black shapes on a blue background) into transparent, tightly cropped PNGs
// under internal/cursor/, where they are embedded into the binary.
//
// The shapes are neutral (white fill, black outline) while the background is
// saturated blue, so "blueness" (B minus the strongest of R/G) separates them
// cleanly, with a soft ramp for anti-aliased edges.
//
// To add a cursor: drop <name>.png in assets/cursor-raw/, add it to `cursors`
// below, and wire the name into internal/cursor.
package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
)

// cursors are the artwork names to process (file name without extension).
var cursors = []string{"arrow", "hand"}

const (
	rawDir = "assets/cursor-raw"
	outDir = "internal/cursor"

	keyStart = 18.0 // blueness where the pixel starts fading out
	keyEnd   = 60.0 // blueness where the pixel is fully background
)

func main() {
	for _, name := range cursors {
		src := load(filepath.Join(rawDir, name+".png"))
		write(name, cropToContent(keyOutBackground(src)))
	}
}

func write(name string, img *image.RGBA) {
	out := filepath.Join(outDir, name+".png")
	if err := writePNG(out, img); err != nil {
		panic(err)
	}
	b := img.Bounds()
	println("wrote", out, b.Dx(), "x", b.Dy())
}

// keyOutBackground removes the blue backdrop, keeping the neutral artwork.
func keyOutBackground(src image.Image) *image.RGBA {
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r16, g16, b16, _ := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			r, g, bl := float64(r16>>8), float64(g16>>8), float64(b16>>8)

			a := 1.0
			switch blueness := bl - max64(r, g); {
			case blueness >= keyEnd:
				a = 0
			case blueness > keyStart:
				a = 1 - (blueness-keyStart)/(keyEnd-keyStart)
			}
			if a <= 0 {
				continue
			}
			lum := clamp255((r + g + bl) / 3) // artwork is neutral: white or black
			i := out.PixOffset(x, y)
			out.Pix[i+0] = byte(lum * a)
			out.Pix[i+1] = byte(lum * a)
			out.Pix[i+2] = byte(lum * a)
			out.Pix[i+3] = byte(255 * a)
		}
	}
	return out
}

func cropToContent(img *image.RGBA) *image.RGBA {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.Pix[img.PixOffset(x, y)+3] > 8 {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if maxX < minX {
		return img
	}
	out := image.NewRGBA(image.Rect(0, 0, maxX-minX+1, maxY-minY+1))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			si := img.PixOffset(x, y)
			di := out.PixOffset(x-minX, y-minY)
			copy(out.Pix[di:di+4], img.Pix[si:si+4])
		}
	}
	return out
}

func load(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		panic(err)
	}
	return img
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func clamp255(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}
