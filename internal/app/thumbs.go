package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	"soteria/internal/infra/store"
	"soteria/internal/infra/thumb"
)

var ErrUnsupported = errors.New("unsupported image")

// Thumbs caches 360px JPEGs of remote images on disk.
type Thumbs struct {
	S   *Session
	Dir string
}

// slots caps concurrent decodes; ponytail: a 24 MP photo takes ~35 MB to decode, a grid asks for dozens at once.
var slots = make(chan struct{}, 4)

// File returns the cached thumbnail for p, keyed by v (its modified time); undecodable formats return ErrUnsupported.
func (t *Thumbs) File(ctx context.Context, p, v string) (string, error) {
	sum := sha256.Sum256([]byte(p + "\x00" + v))
	file := filepath.Join(t.Dir, hex.EncodeToString(sum[:])+".jpg")
	if _, err := os.Stat(file); err == nil {
		return file, nil
	}
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	case <-ctx.Done():
		return "", ctx.Err()
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
	data, err := io.ReadAll(io.LimitReader(resp.Body, 30<<20))
	if err != nil {
		return "", err
	}
	jpg, err := thumb.JPEG(data, 360)
	if err != nil {
		return "", ErrUnsupported
	}
	_ = os.MkdirAll(t.Dir, 0o700)
	if err := store.WriteFile(file, jpg); err != nil {
		return "", err
	}
	return file, nil
}
