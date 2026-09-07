package app

import (
	"context"
	"net/http"

	"soteria/internal/domain"
)

// Files is browsing and the simple edits; every change nudges the search index.
type Files struct {
	S     *Session
	Index *Index
}

func (f *Files) changed(err error) error {
	if err == nil {
		f.Index.ReindexLater()
	}
	return err
}

func (f *Files) List(p string) ([]domain.Entry, error) {
	c, err := f.S.Client()
	if err != nil {
		return nil, err
	}
	return c.List(context.Background(), p)
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
	return f.changed(c.Mkcol(context.Background(), p))
}

func (f *Files) Move(from, to string) error {
	c, err := f.S.Client()
	if err != nil {
		return err
	}
	return f.changed(c.Move(context.Background(), from, to))
}

func (f *Files) Copy(from, to string) error {
	c, err := f.S.Client()
	if err != nil {
		return err
	}
	return f.changed(c.Copy(context.Background(), from, to))
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
