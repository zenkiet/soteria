package app

import (
	"bytes"
	"cmp"
	"context"
	"encoding/gob"
	"maps"
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
	stop     context.CancelFunc
}

type item struct {
	domain.Entry
	// key is fold(Name), precomputed for Search.
	key string
}

// fold drops case and accents so "tai lieu" finds "Tài liệu"; đ has no decomposition, hence the special case.
func fold(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == 'đ':
			return 'd'
		case unicode.Is(unicode.Mn, r):
			return -1
		}
		return r
	}, norm.NFD.String(strings.ToLower(s)))
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
		return store.IndexFile(sv.ID)
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
	var b bytes.Buffer
	if p := i.snapshot(); p != "" && gob.NewEncoder(&b).Encode(list) == nil && os.MkdirAll(filepath.Dir(p), 0o700) == nil {
		_ = store.WriteFile(p, b.Bytes())
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
	ctx, stop := context.WithCancel(context.Background())
	i.stop = stop
	i.mu.Unlock()
	go func() {
		defer stop()
		list, err := c.Tree(ctx, "/", i.emit)
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
			i.Reindex() //nolint:contextcheck // the next crawl owns its context; this one is stopped on return
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
	i.edit(func(m map[string]item) { maps.DeleteFunc(m, func(k string, _ item) bool { return within(k, p) }) })
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

// Clear also stops a crawl in flight so the next server's crawl doesn't queue behind it.
func (i *Index) Clear() {
	i.mu.Lock()
	if i.stop != nil {
		i.stop()
	}
	i.entries, i.at = nil, time.Time{}
	i.mu.Unlock()
}

func (i *Index) emit(s domain.IndexStatus) { i.ev.Emit("index", s) }

func (i *Index) status() domain.IndexStatus {
	s := domain.IndexStatus{Count: len(i.entries), Done: !i.indexing}
	// Before the first crawl the zero time would read as "indexed 17,000,000 h ago".
	if !i.at.IsZero() {
		s.At = i.at.Unix()
	}
	return s
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
		if !it.Dir && !junk(it.Path) {
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

// Search matches names and, when set, the kind ("images", "folders"…) and a modified-since Unix time; filters alone are a query.
// It returns the 500 best: exact names, then names that start with q, then the most recently modified.
func (i *Index) Search(q, kind string, since int64, hidden bool) domain.Found {
	q = fold(strings.TrimSpace(q))
	if q == "" && kind == "" && since == 0 {
		return domain.Found{Entries: []domain.Entry{}}
	}
	type hit struct {
		domain.Entry
		rank int
	}
	var hits []hit
	i.mu.Lock()
	for _, it := range i.entries {
		if strings.Contains(it.key, q) && (kind == "" || kindOf(it.Entry) == kind) && (since == 0 || it.Modified.Unix() >= since) && (hidden || !junk(it.Path)) {
			rank := 2
			if it.key == q {
				rank = 0
			} else if strings.HasPrefix(it.key, q) {
				rank = 1
			}
			hits = append(hits, hit{it.Entry, rank})
		}
	}
	i.mu.Unlock()
	slices.SortFunc(hits, func(a, b hit) int { return cmp.Or(cmp.Compare(a.rank, b.rank), b.Modified.Compare(a.Modified)) })
	out := make([]domain.Entry, min(len(hits), 500))
	for n := range out {
		out[n] = hits[n].Entry
	}
	return domain.Found{Entries: out, Total: len(hits)}
}

// Finder and Dropbox keep these out of sight: whatever is in a dot folder, dotfiles, Office lock files and Windows
// thumbnail caches.
func junk(p string) bool {
	name := path.Base(p)
	return strings.Contains(p, "/.") || strings.HasPrefix(name, "~$") || strings.EqualFold(name, "Thumbs.db") || strings.EqualFold(name, "desktop.ini")
}

var kinds = func() map[string]string {
	m := map[string]string{}
	for kind, exts := range map[string]string{
		"images":        "jpg jpeg png gif webp heic heif bmp tif tiff svg avif",
		"documents":     "pdf doc docx rtf txt md odt pages epub",
		"spreadsheets":  "xls xlsx csv ods numbers",
		"presentations": "ppt pptx key odp",
		"videos":        "mp4 mov m4v mkv webm avi",
		"audio":         "mp3 m4a wav aac flac ogg",
		"archives":      "zip gz tgz tar rar 7z",
	} {
		for _, e := range strings.Fields(exts) {
			m[e] = kind
		}
	}
	return m
}()

func kindOf(e domain.Entry) string {
	if e.Dir {
		return "folders"
	}
	return kinds[strings.ToLower(strings.TrimPrefix(path.Ext(e.Name), "."))]
}

// Bytes sums the sizes of every indexed file.
func (i *Index) Bytes() (n int64) {
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, it := range i.entries {
		if !it.Dir {
			n += it.Size
		}
	}
	return n
}
