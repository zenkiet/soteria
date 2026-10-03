package app

import (
	"cmp"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"soteria/internal/domain"
	"soteria/internal/infra/webdav"
)

type transfer struct {
	domain.Transfer
	cancel context.CancelFunc
	done   chan error
}

type job func(ctx context.Context, progress func(int64)) error

const stallAfter = 60 * time.Second

var errStalled = errors.New("no data for 60 s")

// Transfers is the queue: three at a time, retries on network trouble, progress on the "transfer" event.
type Transfers struct {
	mu       sync.Mutex
	s        *Session
	ev       Events
	index    *Index
	ts       []*transfer
	sem      chan struct{}
	dragging []domain.Entry
	// OnChange fires on "queued" and every final state; the tray derives its badge and notifications from it.
	OnChange func(status, kind string)
}

func NewTransfers(s *Session, idx *Index, ev Events) *Transfers {
	return &Transfers{s: s, ev: ev, index: idx, sem: make(chan struct{}, 3)}
}

func (a *Transfers) changed(status, kind string) {
	if a.OnChange != nil {
		a.OnChange(status, kind)
	}
}

func (a *Transfers) enqueue(t domain.Transfer, run job) *transfer {
	ctx, cancel := context.WithCancel(context.Background())
	if t.ID == "" {
		t.ID = rand.Text()
	}
	t.Status, t.Done, t.Error = "queued", 0, ""
	tr := &transfer{Transfer: t, cancel: cancel, done: make(chan error, 1)}
	a.mu.Lock()
	a.ts = append(a.ts, tr)
	a.mu.Unlock()
	a.emit(tr)
	a.changed("queued", t.Kind)
	go func() {
		a.sem <- struct{}{}
		defer func() { <-a.sem }()
		var err error
		for attempt := 1; ctx.Err() == nil; attempt++ {
			a.update(tr, "running", nil)
			err = a.attempt(ctx, tr, run)
			if err == nil || ctx.Err() != nil || !domain.Retryable(err) || attempt == 3 {
				break
			}
			select {
			case <-ctx.Done():
			case <-time.After(time.Duration(attempt*2) * time.Second):
			}
		}
		switch {
		case ctx.Err() != nil:
			err = ctx.Err()
			a.update(tr, "cancelled", nil)
		case err != nil:
			a.update(tr, "error", err)
		default:
			a.update(tr, "done", nil)
		}
		tr.done <- err
	}()
	return tr
}

// attempt cancels the job once no byte has moved for stallAfter.
func (a *Transfers) attempt(ctx context.Context, tr *transfer, run job) error {
	actx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	stall := time.AfterFunc(stallAfter, func() { cancel(errStalled) })
	defer stall.Stop()
	a.mu.Lock()
	tr.Done = 0
	a.mu.Unlock()
	last := time.Now()
	err := run(actx, func(n int64) {
		stall.Reset(stallAfter)
		a.mu.Lock()
		tr.Done = n
		a.mu.Unlock()
		if time.Since(last) > 200*time.Millisecond {
			last = time.Now()
			a.emit(tr)
		}
	})
	if errors.Is(context.Cause(actx), errStalled) {
		return errStalled
	}
	return err
}

func (a *Transfers) update(tr *transfer, status string, err error) {
	a.mu.Lock()
	tr.Status = status
	if err != nil {
		tr.Error = err.Error()
		slog.Warn("transfer failed", "kind", tr.Kind, "remote", tr.Remote, "err", err)
	}
	if status == "done" {
		tr.Done = tr.Total
	}
	a.mu.Unlock()
	a.emit(tr)
	if status != "running" {
		a.changed(status, tr.Kind)
	}
	if status == "done" && tr.Kind == "upload" {
		a.index.Put(domain.Entry{Name: path.Base(tr.Remote), Path: tr.Remote, Size: tr.Total, Modified: time.Now()})
	}
}

func (a *Transfers) emit(tr *transfer) {
	a.mu.Lock()
	snap := tr.Transfer
	a.mu.Unlock()
	a.ev.Emit("transfer", snap)
}

func (a *Transfers) List() []domain.Transfer {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]domain.Transfer, len(a.ts))
	for i, t := range a.ts {
		out[i] = t.Transfer
	}
	return out
}

// Running counts transfers that are queued or moving.
func (a *Transfers) Running() (n int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, t := range a.ts {
		if t.Status == "queued" || t.Status == "running" {
			n++
		}
	}
	return n
}

func (a *Transfers) Cancel(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, t := range a.ts {
		if t.ID == id {
			t.cancel()
		}
	}
}

func (a *Transfers) CancelGroup(group string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, t := range a.ts {
		if t.Group == group {
			t.cancel()
		}
	}
}

// Retry re-queues a finished transfer under the same ID.
func (a *Transfers) Retry(id string) error {
	c, err := a.s.Client()
	if err != nil {
		return err
	}
	a.mu.Lock()
	i := slices.IndexFunc(a.ts, func(t *transfer) bool { return t.ID == id })
	if i < 0 {
		a.mu.Unlock()
		return nil
	}
	t := a.ts[i].Transfer
	a.ts = slices.Delete(a.ts, i, i+1)
	a.mu.Unlock()
	if t.Kind == "download" {
		a.enqueue(t, fetch(c, t.Remote, t.Local))
	} else {
		a.enqueue(t, upload(c, t.Local, t.Remote, t.Total))
	}
	return nil
}

func (a *Transfers) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ts = slices.DeleteFunc(a.ts, func(t *transfer) bool { return t.Status != "queued" && t.Status != "running" })
}

type counter struct {
	io.Reader
	n int64
	f func(int64)
}

func (c *counter) Read(p []byte) (int, error) {
	n, err := c.Reader.Read(p)
	c.n += int64(n)
	c.f(c.n)
	return n, err
}

// Upload queues files and folder trees into dir; names already present are returned as conflicts instead of overwritten.
func (a *Transfers) Upload(locals []string, dir string) ([]domain.Conflict, error) {
	c, err := a.s.Client()
	if err != nil {
		return nil, err
	}
	existing, err := c.List(context.Background(), dir)
	if err != nil {
		return nil, err
	}
	byName := map[string]domain.Entry{}
	for _, e := range existing {
		byName[e.Name] = e
	}
	conflicts := []domain.Conflict{}
	for _, local := range locals {
		st, err := os.Stat(local)
		if err != nil {
			continue
		}
		remote := path.Join(dir, st.Name())
		if e, ok := byName[st.Name()]; ok {
			conflicts = append(conflicts, domain.Conflict{Local: local, Remote: remote, Dir: st.IsDir(), Size: e.Size, Modified: e.Modified, LocalSize: st.Size(), LocalModified: st.ModTime()})
			continue
		}
		a.put(c, local, remote, st)
	}
	return conflicts, nil
}

// UploadAs resolves a conflict: "replace" and "merge" write onto the existing name, "keep" picks a free one.
func (a *Transfers) UploadAs(local, remote, mode string) error {
	c, err := a.s.Client()
	if err != nil {
		return err
	}
	st, err := os.Stat(local)
	if err != nil {
		return err
	}
	if mode == "keep" {
		existing, err := c.List(context.Background(), path.Dir(remote))
		if err != nil {
			return err
		}
		remote = domain.FreeName(remote, func(p string) bool {
			return slices.ContainsFunc(existing, func(e domain.Entry) bool { return e.Path == p })
		})
	}
	a.put(c, local, remote, st)
	return nil
}

func (a *Transfers) put(c *webdav.Client, local, remote string, st os.FileInfo) {
	if !st.IsDir() {
		a.enqueue(domain.Transfer{Kind: "upload", Name: st.Name(), Remote: remote, Local: local, Total: st.Size()}, upload(c, local, remote, st.Size()))
		return
	}
	go func() {
		_ = filepath.WalkDir(local, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(local, p)
			target := path.Join(remote, filepath.ToSlash(rel))
			if d.IsDir() {
				if c.Mkcol(context.Background(), target) == nil {
					a.index.Put(domain.Entry{Name: path.Base(target), Path: target, Dir: true, Modified: time.Now()})
				}
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			a.enqueue(domain.Transfer{Kind: "upload", Group: remote, Name: filepath.ToSlash(rel), Remote: target, Local: p, Total: info.Size()}, upload(c, p, target, info.Size()))
			return nil
		})
	}()
}

// Adopt queues a staged file the drive couldn't save; it is deleted only once the upload lands.
func (a *Transfers) Adopt(local, remote string, size int64) {
	c, err := a.s.Client()
	if err != nil {
		slog.Error("unsaved drive write kept", "local", local, "err", err)
		return
	}
	run := upload(c, local, remote, size)
	a.enqueue(domain.Transfer{Kind: "upload", Name: path.Base(remote), Remote: remote, Local: local, Total: size}, func(ctx context.Context, progress func(int64)) error {
		if err := run(ctx, progress); err != nil {
			return err
		}
		_ = os.Remove(local)
		return nil
	})
}

func upload(c *webdav.Client, local, remote string, size int64) job {
	return func(ctx context.Context, progress func(int64)) error {
		f, err := os.Open(local)
		if err != nil {
			return err
		}
		defer f.Close()
		return c.Put(ctx, remote, &counter{Reader: f, f: progress}, size)
	}
}

func downloadDir(dest string) string {
	if dest != "" {
		return dest
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Downloads")
}

// localName keeps a server path under the download folder: "..", or "a\b" on Windows, is refused.
func localName(rel string) (string, error) {
	p, err := filepath.Localize(rel)
	if err != nil {
		return "", fmt.Errorf("%q can't be saved on this computer", rel)
	}
	return p, nil
}

func (a *Transfers) Download(remote string, size int64, dest string) (string, error) {
	c, err := a.s.Client()
	if err != nil {
		return "", err
	}
	name, err := localName(path.Base(remote))
	if err != nil {
		return "", err
	}
	local := domain.FreeName(filepath.Join(downloadDir(dest), name), func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	})
	return a.enqueue(domain.Transfer{Kind: "download", Name: path.Base(remote), Remote: remote, Local: local, Total: size}, fetch(c, remote, local)).ID, nil
}

// DownloadDir queues every file under remote into dest/<folder>/…, grouped like a folder upload.
func (a *Transfers) DownloadDir(remote, dest string) error {
	c, err := a.s.Client()
	if err != nil {
		return err
	}
	entries, err := c.Tree(context.Background(), remote, nil)
	if err != nil {
		return err
	}
	name, err := localName(path.Base(remote))
	if err != nil {
		return err
	}
	base := filepath.Join(downloadDir(dest), name)
	_ = os.MkdirAll(base, 0o755)
	for _, e := range entries {
		rel := strings.TrimPrefix(e.Path, remote+"/")
		lrel, err := localName(rel)
		if err != nil {
			continue
		}
		local := filepath.Join(base, lrel)
		if e.Dir {
			_ = os.MkdirAll(local, 0o755)
			continue
		}
		_ = os.MkdirAll(filepath.Dir(local), 0o755)
		a.enqueue(domain.Transfer{Kind: "download", Group: remote, Name: rel, Remote: e.Path, Local: local, Total: e.Size}, fetch(c, e.Path, local))
	}
	return nil
}

// fetch resumes a retry from the .part file while the server still vouches (If-Range) for the same version.
func fetch(c *webdav.Client, remote, local string) job {
	var version string
	return func(ctx context.Context, progress func(int64)) error {
		part := local + ".part"
		var have int64
		var hdr []string
		if st, err := os.Stat(part); err == nil && version != "" {
			have = st.Size()
			hdr = []string{"Range", fmt.Sprintf("bytes=%d-", have), "If-Range", version}
		}
		resp, err := c.Do(ctx, http.MethodGet, remote, nil, 0, hdr...)
		if de, ok := errors.AsType[*domain.DavError](err); ok && de.Code == http.StatusRequestedRangeNotSatisfiable {
			// Same version and nothing past what we have: the part is already complete.
			return os.Rename(part, local)
		}
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		version = cmp.Or(resp.Header.Get("ETag"), resp.Header.Get("Last-Modified"))
		var f *os.File
		if resp.StatusCode == http.StatusPartialContent {
			f, err = os.OpenFile(part, os.O_WRONLY|os.O_APPEND, 0)
		} else {
			have = 0
			f, err = os.Create(part)
		}
		if err != nil {
			return err
		}
		_, err = io.Copy(f, &counter{Reader: resp.Body, n: have, f: progress})
		f.Close()
		if err != nil {
			if ctx.Err() != nil {
				os.Remove(part)
			}
			return err
		}
		return os.Rename(part, local)
	}
}

// SetDragging remembers the entries of the drag in progress so DragWrite knows their sizes.
func (a *Transfers) SetDragging(entries []domain.Entry) {
	a.mu.Lock()
	a.dragging = entries
	a.mu.Unlock()
}

// DragWrite downloads remote to the path the OS chose for a dragged-out file and blocks until done.
func (a *Transfers) DragWrite(remote, local string) error {
	c, err := a.s.Client()
	if err != nil {
		return err
	}
	var size int64
	a.mu.Lock()
	for _, e := range a.dragging {
		if e.Path == remote {
			size = e.Size
		}
	}
	a.mu.Unlock()
	return <-a.enqueue(domain.Transfer{Kind: "download", Name: path.Base(remote), Remote: remote, Local: local, Total: size}, fetch(c, remote, local)).done
}
