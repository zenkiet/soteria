package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
)

var ErrUnsupported = errors.New("unsupported image")

// Thumbs caches 360px JPEGs of remote images on disk.
type Thumbs struct {
	S   *Session
	Dir string
}

// File returns the cached thumbnail for p, making it on first use; v (the file's modified time) keys the cache.
// Formats the standard library can't decode return ErrUnsupported.
func (t *Thumbs) File(ctx context.Context, p, v string) (string, error) {
	sum := sha256.Sum256([]byte(p + "\x00" + v))
	file := filepath.Join(t.Dir, hex.EncodeToString(sum[:])+".jpg")
	if _, err := os.Stat(file); err == nil {
		return file, nil
	}
	c, err := t.S.Client()
	if err != nil {
		return "", err
	}
	resp, err := c.Get(ctx, p)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	img, _, err := image.Decode(io.LimitReader(resp.Body, 30<<20))
	if err != nil {
		return "", ErrUnsupported
	}
	_ = os.MkdirAll(t.Dir, 0o700)
	f, err := os.Create(file)
	if err != nil {
		return "", err
	}
	err = jpeg.Encode(f, shrink(img, 360), &jpeg.Options{Quality: 80})
	f.Close()
	if err != nil {
		os.Remove(file)
		return "", err
	}
	return file, nil
}

// shrink box-samples img down to width w; ponytail: a few samples per output pixel instead of every source pixel.
func shrink(img image.Image, w int) image.Image {
	b := img.Bounds()
	if b.Dx() <= w {
		return img
	}
	h := max(b.Dy()*w/b.Dx(), 1)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	fx, fy := float64(b.Dx())/float64(w), float64(b.Dy())/float64(h)
	step := max(int(fx/3), 1)
	for y := 0; y < h; y++ {
		y0, y1 := b.Min.Y+int(float64(y)*fy), b.Min.Y+int(float64(y+1)*fy)
		for x := 0; x < w; x++ {
			x0, x1 := b.Min.X+int(float64(x)*fx), b.Min.X+int(float64(x+1)*fx)
			var r, g, bl, n uint32
			for sy := y0; sy < y1; sy += step {
				for sx := x0; sx < x1; sx += step {
					cr, cg, cb, _ := img.At(sx, sy).RGBA()
					r, g, bl, n = r+cr, g+cg, bl+cb, n+1
				}
			}
			if n == 0 {
				n = 1
			}
			out.Set(x, y, color.RGBA{uint8(r / n >> 8), uint8(g / n >> 8), uint8(bl / n >> 8), 255})
		}
	}
	return out
}
