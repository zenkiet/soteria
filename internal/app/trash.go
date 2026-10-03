package app

import (
	"context"
	"crypto/rand"
	"io"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"soteria/internal/domain"
	"soteria/internal/infra/webdav"
)

// Trash keeps deletes recoverable: /.trash/<stamp>-<rand>/<name> plus a .origin file naming the folder it came from.
type Trash struct {
	S     *Session
	Index *Index
}

// Trash moves p into the trash and returns the new path.
func (t *Trash) Trash(p string) (string, error) {
	c, err := t.S.Client()
	if err != nil {
		return "", err
	}
	ctx := context.Background()
	bin := path.Join(domain.TrashDir, strconv.FormatInt(time.Now().UnixNano(), 36)+"-"+rand.Text()[:6])
	_ = c.Mkcol(ctx, domain.TrashDir)
	if err := c.Mkcol(ctx, bin); err != nil {
		return "", err
	}
	from := path.Dir(p)
	if err := c.Put(ctx, bin+"/.origin", strings.NewReader(from), int64(len(from))); err != nil {
		return "", err
	}
	to := path.Join(bin, path.Base(p))
	if err := c.Move(ctx, p, to); err != nil {
		return "", err
	}
	t.Index.Remove(p)
	return to, nil
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

// List returns trashed items newest first; ponytail: one LIST + GET per item, batch via PROPFIND infinity past a few hundred.
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
		stamp, err := binTime(bin.Name)
		if err != nil || !bin.Dir {
			continue
		}
		items, err := c.List(ctx, bin.Path)
		if err != nil {
			continue
		}
		for _, e := range items {
			if e.Name != ".origin" {
				out = append(out, domain.TrashItem{Entry: e, From: origin(ctx, c, bin.Path), Deleted: stamp})
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
	// The trash is never indexed, so the restored subtree has to be crawled.
	t.Index.ReindexLater()
	return c.Remove(ctx, bin)
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
		if stamp, err := binTime(bin.Name); err == nil && time.Since(stamp) > 30*24*time.Hour {
			_ = c.Remove(ctx, bin.Path)
		}
	}
}

// Bins from before 0.6 have no random suffix.
func binTime(name string) (time.Time, error) {
	stamp, _, _ := strings.Cut(name, "-")
	n, err := strconv.ParseInt(stamp, 36, 64)
	return time.Unix(0, n), err
}
