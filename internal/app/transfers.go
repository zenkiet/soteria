package app

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"soteria/internal/domain"
	"soteria/internal/infra/webdav"
)

type transfer struct {
	domain.Transfer
	cancel context.CancelFunc
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
	OnChange func(status, kind string) // "queued" and every final state; the tray keeps its badge and notifications from it
}

func NewTransfers(s *Session, idx *Index, ev Events) *Transfers {
	return &Transfers{s: s, ev: ev, index: idx, sem: make(chan struct{}, 3)}
}

func (a *Transfers) changed(status, kind string) {
	if a.OnChange != nil {
		a.OnChange(status, kind)
	}
}

func (a *Transfers) enqueue(t domain.Transfer, run job) string {
	ctx, cancel := context.WithCancel(context.Background())
	if t.ID == "" {
		t.ID = strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	t.Status, t.Done, t.Error = "queued", 0, ""
	tr := &transfer{Transfer: t, cancel: cancel}
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
			a.update(tr, "cancelled", nil)
		case err != nil:
			a.update(tr, "error", err)
		default:
			a.update(tr, "done", nil)
		}
	}()
	return t.ID
}

// attempt runs the job once, cancelling it when no byte moves for stallAfter.
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
		if time.Since(last) > 150*time.Millisecond {
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
		log.Printf("%s %s failed: %v", tr.Kind, tr.Remote, err)
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
		a.index.ReindexLater()
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
				_ = c.Mkcol(context.Background(), target)
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

func (a *Transfers) Download(remote string, size int64, dest string) (string, error) {
	c, err := a.s.Client()
	if err != nil {
		return "", err
	}
	local := domain.FreeName(filepath.Join(downloadDir(dest), path.Base(remote)), func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	})
	return a.enqueue(domain.Transfer{Kind: "download", Name: path.Base(remote), Remote: remote, Local: local, Total: size}, fetch(c, remote, local)), nil
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
	base := filepath.Join(downloadDir(dest), path.Base(remote))
	for _, e := range entries {
		rel := strings.TrimPrefix(e.Path, remote+"/")
		local := filepath.Join(base, filepath.FromSlash(rel))
		if e.Dir {
			_ = os.MkdirAll(local, 0o755)
			continue
		}
		_ = os.MkdirAll(filepath.Dir(local), 0o755)
		a.enqueue(domain.Transfer{Kind: "download", Group: remote, Name: rel, Remote: e.Path, Local: local, Total: e.Size}, fetch(c, e.Path, local))
	}
	return nil
}

func fetch(c *webdav.Client, remote, local string) job {
	return func(ctx context.Context, progress func(int64)) error {
		resp, err := c.Get(ctx, remote)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		part := local + ".part"
		f, err := os.Create(part)
		if err != nil {
			return err
		}
		_, err = io.Copy(f, &counter{Reader: resp.Body, f: progress})
		f.Close()
		if err != nil {
			os.Remove(part)
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
	done := make(chan error, 1)
	run := fetch(c, remote, local)
	a.enqueue(domain.Transfer{Kind: "download", Name: path.Base(remote), Remote: remote, Local: local, Total: size}, func(ctx context.Context, progress func(int64)) error {
		err := run(ctx, progress)
		done <- err
		return err
	})
	return <-done
}
