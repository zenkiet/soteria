package app

import (
	"context"
	"net/http"
	"path"
	"time"

	"soteria/internal/domain"
)

// Files is browsing and the simple edits; every change is written through to the search index.
type Files struct {
	S     *Session
	Index *Index
}

func (f *Files) List(ctx context.Context, p string) ([]domain.Entry, error) {
	c, err := f.S.Client()
	if err != nil {
		return nil, err
	}
	return c.List(ctx, p)
}

func (f *Files) Quota() (domain.Quota, error) {
	c, err := f.S.Client()
	if err != nil {
		return domain.Quota{}, err
	}
	return c.Quota(context.Background())
}

func (f *Files) Mkdir(p string) error {
	c, err := f.S.Client()
	if err != nil {
		return err
	}
	if err := c.Mkcol(context.Background(), p); err != nil {
		return err
	}
	f.Index.Put(domain.Entry{Name: path.Base(p), Path: p, Dir: true, Modified: time.Now()})
	return nil
}

func (f *Files) Move(from, to string) error {
	c, err := f.S.Client()
	if err != nil {
		return err
	}
	if err := c.Move(context.Background(), from, to); err != nil {
		return err
	}
	f.Index.Move(from, to, false)
	return nil
}

func (f *Files) Copy(from, to string) error {
	c, err := f.S.Client()
	if err != nil {
		return err
	}
	if err := c.Copy(context.Background(), from, to); err != nil {
		return err
	}
	f.Index.Move(from, to, true)
	return nil
}

// Ping checks the server is reachable; the offline banner polls it.
func (f *Files) Ping() error {
	c, err := f.S.Client()
	if err != nil {
		return err
	}
	return c.Ping(context.Background())
}

func (f *Files) Link(p string) (string, error) {
	c, err := f.S.Client()
	if err != nil {
		return "", err
	}
	return c.URL(p), nil
}

// Open streams a remote file for the preview endpoint, passing Range through for media.
func (f *Files) Open(ctx context.Context, p, rng string) (*http.Response, error) {
	c, err := f.S.Client()
	if err != nil {
		return nil, err
	}
	var hdr []string
	if rng != "" {
		hdr = append(hdr, "Range", rng)
	}
	return c.Do(ctx, http.MethodGet, p, nil, 0, hdr...)
}
