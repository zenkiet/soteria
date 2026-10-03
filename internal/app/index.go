package app

import (
	"context"
	"encoding/gob"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"soteria/internal/domain"
	"soteria/internal/infra/store"
)

// Index is the in-memory search index: crawled in the background, edited in place for this app's own changes.
type Index struct {
	mu       sync.Mutex
	s        *Session
	ev       Events
	entries  map[string]item
	at       time.Time
	indexing bool
	dirty    bool
	later    *time.Timer
	tell     *time.Timer
}

type item struct {
	domain.Entry
	// key is fold(Name), precomputed for Search.
	key string
}

// fold drops case and accents so "tai lieu" finds "Tài liệu"; đ has no decomposition, hence the special case.
func fold(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		switch {
		case r == 'đ':
			out = append(out, 'd')
		case !unicode.Is(unicode.Mn, r):
			out = append(out, r)
		}
	}
	return string(out)
}

func NewIndex(s *Session, ev Events) *Index { return &Index{s: s, ev: ev} }

func (i *Index) set(list []domain.Entry, at time.Time) {
	m := make(map[string]item, len(list))
	for _, e := range list {
		m[e.Path] = item{e, fold(e.Name)}
	}
	i.mu.Lock()
	i.entries, i.at = m, at
	i.mu.Unlock()
}

func (i *Index) snapshot() string {
	if sv := i.s.Current(); sv != nil {
		return filepath.Join(store.CacheDir(), "index-"+sv.ID+".gob")
	}
	return ""
}

// Restore loads the current server's last crawl from disk so search and Recent answer at launch; the next crawl replaces it.
func (i *Index) Restore() {
	var list []domain.Entry
	f, err := os.Open(i.snapshot())
	if err == nil {
		defer f.Close()
		err = gob.NewDecoder(f).Decode(&list)
	}
	if err != nil {
		i.Clear()
		return
	}
	st, _ := f.Stat()
	i.set(list, st.ModTime())
}

func (i *Index) save(list []domain.Entry) {
	p := i.snapshot()
	if p == "" || os.MkdirAll(filepath.Dir(p), 0o700) != nil {
		return
	}
	f, err := os.CreateTemp(filepath.Dir(p), "*.tmp")
	if err != nil {
		return
	}
	err = gob.NewEncoder(f).Encode(list)
	f.Close()
	if err == nil {
		err = os.Rename(f.Name(), p)
	}
	if err != nil {
		os.Remove(f.Name())
	}
}

func (i *Index) Reindex() {
	c, err := i.s.Client()
	i.mu.Lock()
	if i.indexing || err != nil {
		i.dirty = i.indexing
		i.mu.Unlock()
		return
	}
	i.indexing = true
	i.mu.Unlock()
	go func() {
		list, err := c.Tree(context.Background(), "/", i.emit)
		if cur, _ := i.s.Client(); err == nil && cur == c {
			i.set(list, time.Now())
			go i.save(list)
		}
		i.mu.Lock()
		i.indexing = false
		again := i.dirty
		i.dirty = false
		st := i.status()
		i.mu.Unlock()
		i.emit(st)
		if again {
			i.Reindex()
		}
	}()
}

func debounce(t **time.Timer, d time.Duration, f func()) {
	if *t == nil {
		*t = time.AfterFunc(d, f)
	} else {
		(*t).Reset(d)
	}
}

// ReindexLater folds a burst of changes made outside this app's view (the drive, a restore) into one crawl.
func (i *Index) ReindexLater() {
	i.mu.Lock()
	defer i.mu.Unlock()
	debounce(&i.later, 2*time.Second, i.Reindex)
}

// edit applies this app's own change; a crawl in flight is redone since it may have listed the server before it.
func (i *Index) edit(f func(map[string]item)) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.entries != nil {
		f(i.entries)
	}
	i.dirty = i.dirty || i.indexing
	debounce(&i.tell, time.Second, func() { i.emit(i.Status()) })
}

func within(p, root string) bool { return p == root || strings.HasPrefix(p, root+"/") }

func (i *Index) Put(es ...domain.Entry) {
	i.edit(func(m map[string]item) {
		for _, e := range es {
			m[e.Path] = item{e, fold(e.Name)}
		}
	})
}

// Remove drops p and everything under it.
func (i *Index) Remove(p string) {
	i.edit(func(m map[string]item) {
		for k := range m {
			if within(k, p) {
				delete(m, k)
			}
		}
	})
}

// Move re-keys the subtree at from under to; keep leaves the original in place, as a copy does.
func (i *Index) Move(from, to string, keep bool) {
	i.edit(func(m map[string]item) {
		for k, it := range m {
			if within(k, from) {
				if !keep {
					delete(m, k)
				}
				e := it.Entry
				e.Path = to + strings.TrimPrefix(k, from)
				e.Name = path.Base(e.Path)
				m[e.Path] = item{e, fold(e.Name)}
			}
		}
	})
}

func (i *Index) Clear() {
	i.mu.Lock()
	i.entries, i.at = nil, time.Time{}
	i.mu.Unlock()
}

func (i *Index) emit(s domain.IndexStatus) { i.ev.Emit("index", s) }

func (i *Index) status() domain.IndexStatus {
	return domain.IndexStatus{Count: len(i.entries), Done: !i.indexing, At: i.at.Unix()}
}

func (i *Index) Status() domain.IndexStatus {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.status()
}

func (i *Index) Recent(n int) []domain.Entry {
	i.mu.Lock()
	files := []domain.Entry{}
	for _, it := range i.entries {
		if !it.Dir {
			files = append(files, it.Entry)
		}
	}
	i.mu.Unlock()
	slices.SortFunc(files, func(x, y domain.Entry) int { return y.Modified.Compare(x.Modified) })
	return files[:min(n, len(files))]
}

// Usages sums items and bytes under every direct subfolder of dir in one pass; Known marks folders the index has seen.
func (i *Index) Usages(dir string) map[string]domain.Usage {
	pre := strings.TrimSuffix(path.Clean("/"+dir), "/") + "/"
	out := map[string]domain.Usage{}
	i.mu.Lock()
	defer i.mu.Unlock()
	for p, e := range i.entries {
		rest, ok := strings.CutPrefix(p, pre)
		if !ok {
			continue
		}
		name, _, nested := strings.Cut(rest, "/")
		u := out[pre+name]
		switch {
		case nested:
			u.Items++
			u.Bytes += e.Size
		case e.Dir:
			u.Known = true
		default:
			continue
		}
		out[pre+name] = u
	}
	return out
}

func (i *Index) Search(q string) []domain.Entry {
	q = fold(strings.TrimSpace(q))
	out := []domain.Entry{}
	if q == "" {
		return out
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, it := range i.entries {
		if strings.Contains(it.key, q) {
			out = append(out, it.Entry)
			if len(out) == 500 {
				break
			}
		}
	}
	return out
}
