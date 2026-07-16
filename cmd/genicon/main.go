// Command genicon converts assets/icon-source.png into a multi-resolution
// assets/giant-cursor.ico (and a assets/icon-preview.png for review). It
// auto-crops the source to a centered square around the visible icon and
// resizes with high-quality resampling.
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"os"

	xdraw "golang.org/x/image/draw"
)

const source = "assets/icon-source.png"

func main() {
	src := load(source)
	sq := cropSquare(src)

	sizes := []int{16, 32, 48, 64, 128, 256}
	imgs := make([]*image.RGBA, len(sizes))
	for i, s := range sizes {
		imgs[i] = resize(sq, s)
	}
	if err := writeICO("assets/giant-cursor.ico", imgs); err != nil {
		panic(err)
	}
	// Embeddable copy for the tray (go:embed cannot reach ../../assets).
	if err := writeICO("cmd/giant-cursor/appicon.ico", imgs); err != nil {
		panic(err)
	}
	if err := writePNG("assets/icon-preview.png", resize(sq, 256)); err != nil {
		panic(err)
	}
	println("wrote assets/giant-cursor.ico, cmd/giant-cursor/appicon.ico and assets/icon-preview.png")
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

// cropSquare finds the visible icon (alpha > threshold) and returns a centered
// square crop with a small transparent margin.
func cropSquare(img image.Image) *image.RGBA {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a>>8 > 24 {
				minX, maxX = mn(minX, x), mx(maxX, x)
				minY, maxY = mn(minY, y), mx(maxY, y)
			}
		}
	}
	// Square the bounding box without extra padding: these designs are already
	// framed as an icon tile, so any margin would just shrink them.
	w, h := maxX-minX+1, maxY-minY+1
	side := mx(w, h)
	cx, cy := (minX+maxX)/2, (minY+maxY)/2

	out := image.NewRGBA(image.Rect(0, 0, side, side))
	off := image.Pt(side/2-cx, side/2-cy)
	xdraw.Draw(out, b.Add(off), img, b.Min, xdraw.Src)
	return out
}

func resize(src *image.RGBA, size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// writeICO writes a PNG-compressed .ico (supported on Windows Vista+).
func writeICO(path string, images []*image.RGBA) error {
	pngs := make([][]byte, len(images))
	for i, im := range images {
		var b bytes.Buffer
		if err := png.Encode(&b, im); err != nil {
			return err
		}
		pngs[i] = b.Bytes()
	}

	buf := new(bytes.Buffer)
	le := binary.LittleEndian
	binary.Write(buf, le, uint16(0))
	binary.Write(buf, le, uint16(1))
	binary.Write(buf, le, uint16(len(images)))

	offset := 6 + 16*len(images)
	for i, im := range images {
		sz := im.Bounds().Dx()
		dim := byte(sz)
		if sz >= 256 {
			dim = 0
		}
		buf.WriteByte(dim)
		buf.WriteByte(dim)
		buf.WriteByte(0)
		buf.WriteByte(0)
		binary.Write(buf, le, uint16(1))
		binary.Write(buf, le, uint16(32))
		binary.Write(buf, le, uint32(len(pngs[i])))
		binary.Write(buf, le, uint32(offset))
		offset += len(pngs[i])
	}
	for _, p := range pngs {
		buf.Write(p)
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func mn(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func mx(a, b int) int {
	if a > b {
		return a
	}
	return b
}
