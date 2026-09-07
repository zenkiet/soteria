package mount

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"sync"
	"time"

	"github.com/winfsp/cgofuse/fuse"

	"soteria/internal/domain"
	"soteria/internal/infra/webdav"
)

// davFS exposes the WebDAV tree to WinFsp. Reads are HTTP Range requests; a file opened for
// writing is staged in a temp file and PUT once when it is closed, since WebDAV can't write in place.
type davFS struct {
	fuse.FileSystemBase
	m     *Mounter
	mu    sync.Mutex
	dirs  map[string]cached
	fhs   map[uint64]*fh
	next  uint64
	quota domain.Quota
	qAt   time.Time
}

type cached struct {
	at      time.Time
	entries []domain.Entry
}

type fh struct {
	path  string
	size  int64
	tmp   *os.File
	dirty bool
}

var bg = context.Background()

func newDavFS(m *Mounter) *davFS {
	return &davFS{m: m, dirs: map[string]cached{}, fhs: map[uint64]*fh{}}
}

func errno(err error) int {
	var de *domain.DavError
	if err == nil {
		return 0
	}
	if errors.As(err, &de) {
		switch de.Code {
		case http.StatusNotFound:
			return -fuse.ENOENT
		case http.StatusForbidden:
			return -fuse.EACCES
		case http.StatusPreconditionFailed, http.StatusMethodNotAllowed:
			return -fuse.EEXIST
		}
	}
	return -fuse.EIO
}

func (f *davFS) list(dir string) ([]domain.Entry, error) {
	f.mu.Lock()
	c, ok := f.dirs[dir]
	f.mu.Unlock()
	if ok && time.Since(c.at) < 5*time.Second {
		return c.entries, nil
	}
	cl, err := f.m.client()
	if err != nil {
		return nil, err
	}
	entries, err := cl.List(bg, dir)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	f.dirs[dir] = cached{time.Now(), entries}
	f.mu.Unlock()
	return entries, nil
}

func (f *davFS) forget(p string) {
	f.mu.Lock()
	delete(f.dirs, path.Dir(p))
	f.mu.Unlock()
}

func (f *davFS) lookup(p string) (domain.Entry, error) {
	if p == "/" {
		return domain.Entry{Path: "/", Dir: true}, nil
	}
	f.mu.Lock()
	for _, h := range f.fhs {
		if h.path == p && h.tmp != nil {
			fi, err := h.tmp.Stat()
			f.mu.Unlock()
			if err != nil {
				return domain.Entry{}, err
			}
			return domain.Entry{Name: path.Base(p), Path: p, Size: fi.Size(), Modified: fi.ModTime()}, nil
		}
	}
	f.mu.Unlock()
	entries, err := f.list(path.Dir(p))
	if err != nil {
		return domain.Entry{}, err
	}
	for _, e := range entries {
		if e.Name == path.Base(p) {
			return e, nil
		}
	}
	return domain.Entry{}, &domain.DavError{Code: http.StatusNotFound, Text: "not found"}
}

func stat(e domain.Entry, st *fuse.Stat_t) {
	st.Mode = fuse.S_IFREG | 0o666
	if e.Dir {
		st.Mode = fuse.S_IFDIR | 0o777
	}
	st.Nlink, st.Size = 1, e.Size
	st.Mtim = fuse.NewTimespec(e.Modified)
	st.Ctim, st.Atim = st.Mtim, st.Mtim
}

func (f *davFS) add(h *fh) uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	f.fhs[f.next] = h
	return f.next
}

func (f *davFS) handle(id uint64) *fh {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fhs[id]
}

// mutate runs a server change, drops the cached listing around p and re-indexes.
func (f *davFS) mutate(p string, op func(*webdav.Client) error) int {
	cl, err := f.m.client()
	if err != nil {
		return -fuse.EIO
	}
	err = op(cl)
	f.forget(p)
	if err == nil && f.m.onChange != nil {
		f.m.onChange()
	}
	return errno(err)
}

func (f *davFS) Getattr(p string, st *fuse.Stat_t, fhid uint64) int {
	e, err := f.lookup(p)
	if err != nil {
		return errno(err)
	}
	stat(e, st)
	return 0
}

func (f *davFS) Readdir(p string, fill func(string, *fuse.Stat_t, int64) bool, ofst int64, fhid uint64) int {
	entries, err := f.list(p)
	if err != nil {
		return errno(err)
	}
	fill(".", nil, 0)
	fill("..", nil, 0)
	for _, e := range entries {
		var st fuse.Stat_t
		stat(e, &st)
		fill(e.Name, &st, 0)
	}
	return 0
}

func (f *davFS) Statfs(p string, st *fuse.Statfs_t) int {
	f.mu.Lock()
	stale := time.Since(f.qAt) > 30*time.Second
	f.mu.Unlock()
	if stale {
		if cl, cerr := f.m.client(); cerr == nil {
			if q, err := cl.Quota(bg); err == nil {
				f.mu.Lock()
				f.quota, f.qAt = q, time.Now()
				f.mu.Unlock()
			}
		}
	}
	f.mu.Lock()
	used, avail := max(f.quota.Used, 0), f.quota.Available
	f.mu.Unlock()
	if avail < 0 {
		avail = 1 << 50
	}
	st.Bsize, st.Frsize = 4096, 4096
	st.Blocks = uint64(used+avail) / 4096
	st.Bfree, st.Bavail = uint64(avail)/4096, uint64(avail)/4096
	return 0
}

func (f *davFS) Open(p string, flags int) (int, uint64) {
	e, err := f.lookup(p)
	if err != nil {
		return errno(err), ^uint64(0)
	}
	if e.Dir {
		return -fuse.EISDIR, ^uint64(0)
	}
	h := &fh{path: p, size: e.Size}
	if flags&fuse.O_ACCMODE != fuse.O_RDONLY {
		tmp, err := os.CreateTemp("", "soteria-*")
		if err != nil {
			return -fuse.EIO, ^uint64(0)
		}
		if flags&fuse.O_TRUNC != 0 {
			h.dirty = true
		} else if err := f.download(p, tmp); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return errno(err), ^uint64(0)
		}
		h.tmp = tmp
	}
	return 0, f.add(h)
}

func (f *davFS) Create(p string, flags int, mode uint32) (int, uint64) {
	tmp, err := os.CreateTemp("", "soteria-*")
	if err != nil {
		return -fuse.EIO, ^uint64(0)
	}
	return 0, f.add(&fh{path: p, tmp: tmp, dirty: true})
}

func (f *davFS) download(p string, w io.Writer) error {
	cl, err := f.m.client()
	if err != nil {
		return err
	}
	resp, err := cl.Get(bg, p)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(w, resp.Body)
	return err
}

func (f *davFS) Read(p string, buf []byte, ofst int64, fhid uint64) int {
	h := f.handle(fhid)
	if h == nil {
		return -fuse.EIO
	}
	if h.tmp != nil {
		n, err := h.tmp.ReadAt(buf, ofst)
		if err != nil && err != io.EOF {
			return -fuse.EIO
		}
		return n
	}
	if ofst >= h.size {
		return 0
	}
	end := min(ofst+int64(len(buf)), h.size) - 1
	cl, err := f.m.client()
	if err != nil {
		return -fuse.EIO
	}
	resp, err := cl.Do(bg, http.MethodGet, p, nil, 0, "Range", fmt.Sprintf("bytes=%d-%d", ofst, end))
	if err != nil {
		return errno(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && ofst > 0 {
		return -fuse.EIO
	}
	n, err := io.ReadFull(resp.Body, buf[:end-ofst+1])
	if err != nil && err != io.ErrUnexpectedEOF {
		return -fuse.EIO
	}
	return n
}

func (f *davFS) Write(p string, buf []byte, ofst int64, fhid uint64) int {
	h := f.handle(fhid)
	if h == nil || h.tmp == nil {
		return -fuse.EACCES
	}
	n, err := h.tmp.WriteAt(buf, ofst)
	if err != nil {
		return -fuse.EIO
	}
	h.dirty = true
	return n
}

func (f *davFS) Truncate(p string, size int64, fhid uint64) int {
	h := f.handle(fhid)
	if h == nil || h.tmp == nil {
		return -fuse.EACCES
	}
	if err := h.tmp.Truncate(size); err != nil {
		return -fuse.EIO
	}
	h.dirty = true
	return 0
}

func (f *davFS) Release(p string, fhid uint64) int {
	f.mu.Lock()
	h := f.fhs[fhid]
	delete(f.fhs, fhid)
	f.mu.Unlock()
	if h == nil || h.tmp == nil {
		return 0
	}
	defer os.Remove(h.tmp.Name())
	defer h.tmp.Close()
	if !h.dirty {
		return 0
	}
	fi, err := h.tmp.Stat()
	if err != nil {
		return -fuse.EIO
	}
	if _, err := h.tmp.Seek(0, io.SeekStart); err != nil {
		return -fuse.EIO
	}
	return f.mutate(h.path, func(c *webdav.Client) error { return c.Put(bg, h.path, h.tmp, fi.Size()) })
}

func (f *davFS) Mkdir(p string, mode uint32) int {
	return f.mutate(p, func(c *webdav.Client) error { return c.Mkcol(bg, p) })
}

func (f *davFS) Unlink(p string) int {
	return f.mutate(p, func(c *webdav.Client) error { return c.Remove(bg, p) })
}

func (f *davFS) Rmdir(p string) int { return f.Unlink(p) }

func (f *davFS) Rename(from, to string) int {
	f.forget(to)
	return f.mutate(from, func(c *webdav.Client) error { return c.Move(bg, from, to) })
}

func (f *davFS) Opendir(string) (int, uint64)        { return 0, ^uint64(0) }
func (f *davFS) Releasedir(string, uint64) int       { return 0 }
func (f *davFS) Flush(string, uint64) int            { return 0 }
func (f *davFS) Fsync(string, bool, uint64) int      { return 0 }
func (f *davFS) Access(string, uint32) int           { return 0 }
func (f *davFS) Chmod(string, uint32) int            { return 0 }
func (f *davFS) Chown(string, uint32, uint32) int    { return 0 }
func (f *davFS) Utimens(string, []fuse.Timespec) int { return 0 }
