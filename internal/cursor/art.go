package cursor

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
	"sync"

	xdraw "golang.org/x/image/draw"
)

// Cursor artwork: white shapes with a thick black outline on a transparent
// background, tightly cropped. Regenerate from the source art in
// assets/cursor-raw/ with `go run ./cmd/genarrow`.
var (
	//go:embed arrow.png
	arrowPNG []byte
	//go:embed hand.png
	handPNG []byte

	arrowArt = &artwork{data: arrowPNG}
	handArt  = &artwork{data: handPNG}
)

// artFill is the fraction of the cursor box the artwork occupies. The real
// Windows cursors fill only ~62% of their 32px box, so matching that keeps the
// crisp cursors the same visual size as the upscaled system ones.
const artFill = 0.62

// artwork is a high-resolution cursor image scaled down to the requested cursor
// size. Downscaling stays crisp, unlike upscaling the 32px system cursors.
type artwork struct {
	data []byte
	once sync.Once
	img  image.Image
	tipX int // hotspot in source pixels
	tipY int
}

func (a *artwork) load() {
	img, err := png.Decode(bytes.NewReader(a.data))
	if err != nil {
		return
	}
	a.img = img
	a.tipX, a.tipY = findTip(img)
}

// findTip returns the cursor's point — the middle of the topmost opaque run.
// That is the arrow's apex and the hand's index fingertip.
func findTip(img image.Image) (int, int) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		first, last := -1, -1
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a>>8 > 24 {
				if first < 0 {
					first = x
				}
				last = x
			}
		}
		if first >= 0 {
			return (first+last)/2 - b.Min.X, y - b.Min.Y
		}
	}
	return 0, 0
}

// bitmap renders the artwork into a size×size top-down 32bpp premultiplied BGRA
// buffer, anchored at the top-left, and returns the hotspot.
func (a *artwork) bitmap(size int) (buf []byte, hotX, hotY int) {
	if size < 1 {
		size = 1
	}
	buf = make([]byte, size*size*4)
	a.once.Do(a.load)
	if a.img == nil {
		return buf, 0, 0
	}

	sb := a.img.Bounds()
	targetH := int(float64(size) * artFill)
	if targetH < 1 {
		targetH = 1
	}
	targetW := targetH * sb.Dx() / sb.Dy()
	if targetW < 1 {
		targetW = 1
	}
	if targetW > size {
		targetW = size
	}

	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(dst, image.Rect(0, 0, targetW, targetH), a.img, sb, xdraw.Over, nil)

	// image.RGBA is premultiplied RGBA; Windows wants premultiplied BGRA.
	for i := 0; i < size*size; i++ {
		buf[i*4+0] = dst.Pix[i*4+2] // B
		buf[i*4+1] = dst.Pix[i*4+1] // G
		buf[i*4+2] = dst.Pix[i*4+0] // R
		buf[i*4+3] = dst.Pix[i*4+3] // A
	}

	s := float64(targetH) / float64(sb.Dy())
	return buf, int(float64(a.tipX) * s), int(float64(a.tipY) * s)
}
