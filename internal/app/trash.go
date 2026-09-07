package app

import (
	"context"
	"io"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"soteria/internal/domain"
	"soteria/internal/infra/webdav"
)

// Trash keeps deletes recoverable: /.trash/<stamp>/<name> plus a .origin file naming the folder it came from.
type Trash struct {
	S     *Session
	Index *Index
}

func (t *Trash) changed(err error) error {
	if err == nil {
		t.Index.ReindexLater()
	}
	return err
}

// Trash moves p into the trash and returns the new path.
func (t *Trash) Trash(p string) (string, error) {
	c, err := t.S.Client()
	if err != nil {
		return "", err
	}
	ctx := context.Background()
	bin := path.Join(domain.TrashDir, strconv.FormatInt(time.Now().UnixNano(), 36))
	_ = c.Mkcol(ctx, domain.TrashDir)
	if err := c.Mkcol(ctx, bin); err != nil {
		return "", err
	}
	from := path.Dir(p)
	if err := c.Put(ctx, bin+"/.origin", strings.NewReader(from), int64(len(from))); err != nil {
		return "", err
	}
	to := path.Join(bin, path.Base(p))
	return to, t.changed(c.Move(ctx, p, to))
}

func origin(ctx context.Context, c *webdav.Client, bin string) string {
	resp, err := c.Get(ctx, bin+"/.origin")
	if err != nil {
		return "/"
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

// List returns trashed items newest first.
// ponytail: one LIST + one GET per item; batch via PROPFIND infinity if trashes grow past a few hundred.
func (t *Trash) List() ([]domain.TrashItem, error) {
	c, err := t.S.Client()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	_ = c.Mkcol(ctx, domain.TrashDir)
	bins, err := c.List(ctx, domain.TrashDir)
	if err != nil {
		return nil, err
	}
	out := []domain.TrashItem{}
	for _, bin := range bins {
		stamp, err := strconv.ParseInt(bin.Name, 36, 64)
		if err != nil || !bin.Dir {
			continue
		}
		items, err := c.List(ctx, bin.Path)
		if err != nil {
			continue
		}
		for _, e := range items {
			if e.Name != ".origin" {
				out = append(out, domain.TrashItem{Entry: e, From: origin(ctx, c, bin.Path), Deleted: time.Unix(0, stamp)})
			}
		}
	}
	slices.SortFunc(out, func(x, y domain.TrashItem) int { return y.Deleted.Compare(x.Deleted) })
	return out, nil
}

// Restore moves a trashed item back where it came from, recreating the folder and picking a free name if needed.
func (t *Trash) Restore(p string) error {
	c, err := t.S.Client()
	if err != nil {
		return err
	}
	ctx := context.Background()
	bin := path.Dir(p)
	from := origin(ctx, c, bin)
	c.Mkdirs(ctx, from)
	existing, _ := c.List(ctx, from)
	to := domain.FreeName(path.Join(from, path.Base(p)), func(q string) bool {
		return slices.ContainsFunc(existing, func(e domain.Entry) bool { return e.Path == q })
	})
	if err := c.Move(ctx, p, to); err != nil {
		return err
	}
	return t.changed(c.Remove(ctx, bin))
}

// Purge deletes one trashed item for good.
func (t *Trash) Purge(p string) error {
	c, err := t.S.Client()
	if err != nil {
		return err
	}
	return c.Remove(context.Background(), path.Dir(p))
}

func (t *Trash) Empty() error {
	c, err := t.S.Client()
	if err != nil {
		return err
	}
	return c.Remove(context.Background(), domain.TrashDir)
}

// Sweep removes bins older than 30 days.
func (t *Trash) Sweep(c *webdav.Client) {
	ctx := context.Background()
	bins, _ := c.List(ctx, domain.TrashDir)
	for _, bin := range bins {
		if stamp, err := strconv.ParseInt(bin.Name, 36, 64); err == nil && time.Since(time.Unix(0, stamp)) > 30*24*time.Hour {
			_ = c.Remove(ctx, bin.Path)
		}
	}
}
