//go:build !darwin

package thumb

import (
	"bytes"
	"cmp"
	"errors"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	"image/png"

	_ "golang.org/x/image/webp"
)

var errTooBig = errors.New("image too large to preview")

// Make returns data as an image at most px pixels on its longer side.
func Make(data []byte, px int) ([]byte, error) {
	// Decoders allocate the whole canvas from the header: a 1 MB PNG can claim gigabytes.
	if c, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil || c.Width*c.Height > 64<<20 {
		return nil, cmp.Or(err, errTooBig)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	small := shrink(img, px)
	// JPEG has no alpha: transparent icons would turn into black blocks.
	if o, ok := small.(interface{ Opaque() bool }); ok && !o.Opaque() {
		err = png.Encode(&b, small)
	} else {
		err = jpeg.Encode(&b, small, &jpeg.Options{Quality: 80})
	}
	return b.Bytes(), err
}

// ponytail: a few samples per output pixel instead of every source pixel.
func shrink(img image.Image, px int) image.Image {
	b := img.Bounds()
	if max(b.Dx(), b.Dy()) <= px {
		return img
	}
	w, h := px, max(b.Dy()*px/b.Dx(), 1)
	if b.Dy() > b.Dx() {
		w, h = max(b.Dx()*px/b.Dy(), 1), px
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	fx, fy := float64(b.Dx())/float64(w), float64(b.Dy())/float64(h)
	step := max(int(fx/3), 1)
	for y := range h {
		y0, y1 := b.Min.Y+int(float64(y)*fy), b.Min.Y+int(float64(y+1)*fy)
		for x := range w {
			x0, x1 := b.Min.X+int(float64(x)*fx), b.Min.X+int(float64(x+1)*fx)
			var r, g, bl, a, n uint32
			for sy := y0; sy < y1; sy += step {
				for sx := x0; sx < x1; sx += step {
					cr, cg, cb, ca := img.At(sx, sy).RGBA()
					r, g, bl, a, n = r+cr, g+cg, bl+cb, a+ca, n+1
				}
			}
			if n == 0 {
				n = 1
			}
			out.Set(x, y, color.RGBA{uint8(r / n >> 8), uint8(g / n >> 8), uint8(bl / n >> 8), uint8(a / n >> 8)})
		}
	}
	return out
}
