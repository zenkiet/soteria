package mount

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/winfsp/cgofuse/fuse"

	"soteria/internal/domain"
	"soteria/internal/infra/webdav"
)

// WebDAV can't write in place, so writes go to a local copy that the last writer's close uploads.
type davFS struct {
	fuse.FileSystemBase
	m      *Mounter
	mu     sync.Mutex
	dirs   map[string]*cached
	fhs    map[uint64]*fh
	staged map[string]*stage
	next   uint64
	quota  domain.Quota
	qAt    time.Time
}

type cached struct {
	at      time.Time
	entries []domain.Entry
	err     error
	busy    bool
	// ready closes when the first listing lands; later refreshes serve the previous one meanwhile.
	ready chan struct{}
}

// stage is the local copy every handle writing one file shares; io serialises filling and uploading it, davFS.mu guards the rest.
type stage struct {
	io    sync.Mutex
	path  string
	tmp   *os.File
	refs  int
	dirty bool
}

type fh struct {
	size  int64
	write bool
	// st is the copy this handle writes, or reads because it was opened before the upload landed.
	st *stage
	mu sync.Mutex
	// ahead is the read-ahead window starting at offset at.
	ahead []byte
	at    int64
}

const readAhead = 1 << 20

var bg = context.Background()

func newDavFS(m *Mounter) *davFS {
	return &davFS{m: m, dirs: map[string]*cached{}, fhs: map[uint64]*fh{}, staged: map[string]*stage{}, quota: domain.Quota{Used: -1, Available: -1}}
}

func errno(err error) int {
	if err == nil {
		return 0
	}
	if de, ok := errors.AsType[*domain.DavError](err); ok {
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

// Explorer probes a folder from many threads at once, so they share one PROPFIND and only the first listing waits.
func (f *davFS) list(dir string) ([]domain.Entry, error) {
	f.mu.Lock()
	c := f.dirs[dir]
	switch {
	case c == nil:
		c = &cached{busy: true, ready: make(chan struct{})}
		f.dirs[dir] = c
		go f.fetch(dir, c)
	case !c.busy && time.Since(c.at) > 5*time.Second:
		c.busy = true
		go f.fetch(dir, c)
	}
	f.mu.Unlock()
	<-c.ready
	f.mu.Lock()
	defer f.mu.Unlock()
	return c.entries, c.err
}

func (f *davFS) fetch(dir string, c *cached) {
	cl, err := f.m.client()
	var entries []domain.Entry
	if err == nil {
		ctx, cancel := context.WithTimeout(bg, time.Minute)
		entries, err = cl.List(ctx, dir)
		cancel()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	c.at, c.entries, c.err, c.busy = time.Now(), entries, err, false
	select {
	case <-c.ready:
	default:
		close(c.ready)
	}
}

func (f *davFS) forget(p string) {
	f.mu.Lock()
	delete(f.dirs, path.Dir(p))
	f.mu.Unlock()
}

func (f *davFS) pending(match func(p string) bool) []domain.Entry {
	f.mu.Lock()
	tmps := map[string]*os.File{}
	for p, s := range f.staged {
		if s.tmp != nil && match(p) {
			tmps[p] = s.tmp
		}
	}
	f.mu.Unlock()
	var out []domain.Entry
	for p, t := range tmps {
		if fi, err := t.Stat(); err == nil {
			out = append(out, domain.Entry{Name: path.Base(p), Path: p, Size: fi.Size(), Modified: fi.ModTime()})
		}
	}
	return out
}

func (f *davFS) lookup(p string) (domain.Entry, error) {
	if p == "/" {
		return domain.Entry{Path: "/", Dir: true}, nil
	}
	if l := f.pending(func(q string) bool { return q == p }); len(l) > 0 {
		return l[0], nil
	}
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
	st.Birthtim = fuse.NewTimespec(cmp.Or(e.Created, e.Modified))
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

func (f *davFS) mutate(p string, op func(*webdav.Client) error, after func(Index)) int {
	cl, err := f.m.client()
	if err != nil {
		return -fuse.EIO
	}
	err = op(cl)
	f.forget(p)
	if err == nil && f.m.index != nil {
		after(f.m.index)
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
	seen := map[string]bool{}
	for _, e := range append(f.pending(func(q string) bool { return path.Dir(q) == p }), entries...) {
		if !seen[e.Name] {
			seen[e.Name] = true
			var st fuse.Stat_t
			stat(e, &st)
			fill(e.Name, &st, 0)
		}
	}
	return 0
}

// Windows asks on every view of This PC and every file dialog, so this never waits on the server.
func (f *davFS) Statfs(p string, st *fuse.Statfs_t) int {
	f.mu.Lock()
	if time.Since(f.qAt) > 30*time.Second {
		f.qAt = time.Now()
		go func() {
			if cl, err := f.m.client(); err == nil {
				if q, err := cl.Quota(bg); err == nil {
					f.mu.Lock()
					f.quota = q
					f.mu.Unlock()
				}
			}
		}()
	}
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
	h := &fh{size: e.Size, write: flags&fuse.O_ACCMODE != fuse.O_RDONLY}
	f.mu.Lock()
	if h.st = f.staged[p]; h.st != nil && h.write {
		h.st.refs++
	}
	f.mu.Unlock()
	return 0, f.add(h)
}

func (f *davFS) Create(p string, flags int, mode uint32) (int, uint64) {
	tmp, err := os.CreateTemp("", "soteria-*")
	if err != nil {
		return -fuse.EIO, ^uint64(0)
	}
	s := &stage{path: p, tmp: tmp, refs: 1, dirty: true}
	f.mu.Lock()
	f.staged[p] = s
	f.mu.Unlock()
	return 0, f.add(&fh{write: true, st: s})
}

// The copy comes back locked: a write landing mid-upload would be marked saved without being sent.
func (f *davFS) attach(h *fh, p string, keep bool) (*stage, error) {
	f.mu.Lock()
	if h.st == nil {
		if h.st = f.staged[p]; h.st == nil {
			h.st = &stage{path: p}
			f.staged[p] = h.st
		}
		h.st.refs++
	}
	s := h.st
	f.mu.Unlock()
	s.io.Lock()
	f.mu.Lock()
	ready := s.tmp != nil
	f.mu.Unlock()
	if ready {
		return s, nil
	}
	tmp, err := os.CreateTemp("", "soteria-*")
	if err == nil && keep {
		if err = f.download(p, tmp); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}
	if err != nil {
		s.io.Unlock()
		return nil, err
	}
	f.mu.Lock()
	s.tmp = tmp
	f.mu.Unlock()
	return s, nil
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
	f.mu.Lock()
	var tmp *os.File
	if h.st != nil {
		tmp = h.st.tmp
	}
	f.mu.Unlock()
	if tmp != nil {
		n, err := tmp.ReadAt(buf, ofst)
		if err == nil || err == io.EOF {
			return n
		}
		// A copy only closes once its upload landed, so the server has the file.
		if !errors.Is(err, os.ErrClosed) {
			return -fuse.EIO
		}
	}
	if ofst >= h.size {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if ofst < h.at || min(ofst+int64(len(buf)), h.size) > h.at+int64(len(h.ahead)) {
		end := min(ofst+max(int64(len(buf)), readAhead), h.size) - 1
		cl, err := f.m.client()
		if err != nil {
			return -fuse.EIO
		}
		// A stalled server would otherwise hold this WinFsp thread forever.
		ctx, cancel := context.WithTimeout(bg, time.Minute)
		defer cancel()
		resp, err := cl.Do(ctx, http.MethodGet, p, nil, 0, "Range", fmt.Sprintf("bytes=%d-%d", ofst, end))
		if err != nil {
			return errno(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusPartialContent && ofst > 0 {
			return -fuse.EIO
		}
		data := make([]byte, end-ofst+1)
		n, err := io.ReadFull(resp.Body, data)
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
			return -fuse.EIO
		}
		h.ahead, h.at = data[:n], ofst
	}
	return copy(buf, h.ahead[ofst-h.at:])
}

func (f *davFS) Write(p string, buf []byte, ofst int64, fhid uint64) int {
	h := f.handle(fhid)
	if h == nil || !h.write {
		return -fuse.EACCES
	}
	s, err := f.attach(h, p, true)
	if err != nil {
		return errno(err)
	}
	defer s.io.Unlock()
	n, err := s.tmp.WriteAt(buf, ofst)
	if err != nil {
		return -fuse.EIO
	}
	f.mu.Lock()
	s.dirty = true
	f.mu.Unlock()
	return n
}

// Windows overwrites a file by truncating it to zero, which needs nothing from the server.
func (f *davFS) Truncate(p string, size int64, fhid uint64) int {
	h := f.handle(fhid)
	if h == nil || !h.write {
		return -fuse.EACCES
	}
	s, err := f.attach(h, p, size > 0)
	if err != nil {
		return errno(err)
	}
	defer s.io.Unlock()
	if err := s.tmp.Truncate(size); err != nil {
		return -fuse.EIO
	}
	f.mu.Lock()
	s.dirty = true
	f.mu.Unlock()
	return 0
}

func (f *davFS) Release(p string, fhid uint64) int {
	f.mu.Lock()
	h := f.fhs[fhid]
	delete(f.fhs, fhid)
	var s *stage
	if h != nil && h.write && h.st != nil {
		if h.st.refs--; h.st.refs == 0 {
			s = h.st
		}
	}
	f.mu.Unlock()
	if s == nil {
		return 0
	}
	return f.save(s)
}

// The copy stays staged through the upload so the file doesn't vanish from the drive meanwhile.
func (f *davFS) save(s *stage) int {
	s.io.Lock()
	defer s.io.Unlock()
	f.mu.Lock()
	p, tmp, put := s.path, s.tmp, s.dirty && f.staged[s.path] == s
	f.mu.Unlock()
	rc, size := 0, int64(-1)
	if put {
		rc = -fuse.EIO
		if fi, err := tmp.Stat(); err == nil {
			size = fi.Size()
			rc = f.mutate(p, func(c *webdav.Client) error { return c.Put(bg, p, io.NewSectionReader(tmp, 0, size), size) }, func(x Index) {
				x.Put(domain.Entry{Name: path.Base(p), Path: p, Size: size, Modified: time.Now()})
			})
		}
	}
	f.mu.Lock()
	// A writer that opened it during the upload saves it again when it closes.
	if s.refs > 0 {
		s.dirty = s.dirty && rc != 0
		f.mu.Unlock()
		return rc
	}
	if f.staged[p] == s {
		delete(f.staged, p)
	}
	f.mu.Unlock()
	if tmp == nil {
		return rc
	}
	tmp.Close()
	switch {
	case rc == 0:
		os.Remove(tmp.Name())
	case size >= 0 && f.m.Adopt != nil:
		f.m.Adopt(tmp.Name(), p, size)
	}
	return rc
}

// Unstaged copies don't upload on close; they come back locked, after any upload in flight, so none starts until the caller restages them.
func (f *davFS) unstage(p string) []*stage {
	f.mu.Lock()
	var out []*stage
	for q, s := range f.staged {
		if q == p || strings.HasPrefix(q, p+"/") {
			delete(f.staged, q)
			out = append(out, s)
		}
	}
	f.mu.Unlock()
	for _, s := range out {
		s.io.Lock()
	}
	return out
}

func (f *davFS) restage(ss []*stage, from, to string) {
	f.mu.Lock()
	for _, s := range ss {
		s.path = to + strings.TrimPrefix(s.path, from)
		f.staged[s.path] = s
	}
	f.mu.Unlock()
	unlock(ss)
}

func unlock(ss []*stage) {
	for _, s := range ss {
		s.io.Unlock()
	}
}

func (f *davFS) Mkdir(p string, mode uint32) int {
	return f.mutate(p, func(c *webdav.Client) error { return c.Mkcol(bg, p) }, func(x Index) {
		x.Put(domain.Entry{Name: path.Base(p), Path: p, Dir: true, Modified: time.Now()})
	})
}

func (f *davFS) Unlink(p string) int {
	ss := f.unstage(p)
	unlock(ss)
	rc := f.mutate(p, func(c *webdav.Client) error { return c.Remove(bg, p) }, func(x Index) { x.Remove(p) })
	// A new file still on its way up was never on the server.
	if rc == -fuse.ENOENT && len(ss) > 0 {
		return 0
	}
	return rc
}

func (f *davFS) Rmdir(p string) int { return f.Unlink(p) }

// WinFsp already refused the rename if the target exists and the caller didn't ask to replace it.
func (f *davFS) Rename(from, to string) int {
	// Overwriting a folder on the server would delete everything in it.
	if e, err := f.lookup(to); err == nil && e.Dir {
		return -fuse.EEXIST
	}
	// Held through the MOVE: a copy closed meanwhile must upload under the name that wins.
	ss, old := f.unstage(from), f.unstage(to)
	f.forget(to)
	rc := f.mutate(from, func(c *webdav.Client) error { return c.Replace(bg, from, to) }, func(x Index) { x.Move(from, to, false) })
	if rc == 0 || rc == -fuse.ENOENT && len(ss) > 0 {
		f.restage(ss, from, to)
		unlock(old)
		return 0
	}
	f.restage(ss, from, from)
	f.restage(old, to, to)
	return rc
}

func (f *davFS) Releasedir(string, uint64) int       { return 0 }
func (f *davFS) Flush(string, uint64) int            { return 0 }
func (f *davFS) Access(string, uint32) int           { return 0 }
func (f *davFS) Chmod(string, uint32) int            { return 0 }
func (f *davFS) Chown(string, uint32, uint32) int    { return 0 }
func (f *davFS) Utimens(string, []fuse.Timespec) int { return 0 }
