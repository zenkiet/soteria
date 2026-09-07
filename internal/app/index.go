package app

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"soteria/internal/domain"
)

// Index is the in-memory search index, rebuilt in the background and reported on the "index" event.
type Index struct {
	mu       sync.Mutex
	s        *Session
	ev       Events
	entries  []domain.Entry
	at       time.Time
	indexing bool
	dirty    bool
	later    *time.Timer
}

func NewIndex(s *Session, ev Events) *Index { return &Index{s: s, ev: ev} }

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
		entries, err := c.Tree(context.Background(), "/", i.emit)
		cur, _ := i.s.Client()
		i.mu.Lock()
		if err == nil && cur == c {
			i.entries, i.at = entries, time.Now()
		}
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

// ReindexLater folds a burst of changes into one crawl two seconds after the last one.
func (i *Index) ReindexLater() {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.later == nil {
		i.later = time.AfterFunc(2*time.Second, i.Reindex)
	} else {
		i.later.Reset(2 * time.Second)
	}
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
	defer i.mu.Unlock()
	files := slices.DeleteFunc(slices.Clone(i.entries), func(e domain.Entry) bool { return e.Dir })
	slices.SortFunc(files, func(x, y domain.Entry) int { return y.Modified.Compare(x.Modified) })
	if len(files) > n {
		files = files[:n]
	}
	return files
}

func (i *Index) Search(q string) []domain.Entry {
	q = strings.ToLower(strings.TrimSpace(q))
	out := []domain.Entry{}
	if q == "" {
		return out
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, e := range i.entries {
		if strings.Contains(strings.ToLower(e.Name), q) {
			out = append(out, e)
			if len(out) == 500 {
				break
			}
		}
	}
	return out
}
