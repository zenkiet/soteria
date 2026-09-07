package app

import (
	"fmt"
	"sync"
)

// Background is the "keep running when the window closes" policy: the two switches, the quit guard,
// and the tally that becomes one notification when a queue finishes.
type Background struct {
	mu     sync.Mutex
	on     bool
	notify bool
	force  bool
	batch  map[string]int // done uploads / downloads / errors since the queue last drained
	T      *Transfers
}

func NewBackground(t *Transfers) *Background { return &Background{batch: map[string]int{}, T: t} }

func (b *Background) Set(on, notify bool) {
	b.mu.Lock()
	b.on, b.notify = on, notify
	b.mu.Unlock()
}

func (b *Background) Enabled() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.on
}

// CanQuit is true once nothing is running or the user confirmed quitting anyway.
func (b *Background) CanQuit() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.force || b.T.Running() == 0
}

func (b *Background) ForceQuit() {
	b.mu.Lock()
	b.force = true
	b.mu.Unlock()
}

// Finished tallies a terminal transfer; when the queue drains it hands back the notification text, once.
func (b *Background) Finished(status, kind string) (title, body string, ok bool) {
	b.mu.Lock()
	switch status {
	case "done":
		b.batch[kind]++
	case "error":
		b.batch["error"]++
	}
	b.mu.Unlock()
	if b.T.Running() > 0 {
		return "", "", false
	}
	b.mu.Lock()
	batch, notify := b.batch, b.notify
	b.batch = map[string]int{}
	b.mu.Unlock()
	if !notify || len(batch) == 0 {
		return "", "", false
	}
	up, down, bad := batch["upload"], batch["download"], batch["error"]
	title = Plural(up+down, "transfer") + " finished"
	switch {
	case up+down == 0:
		title = Plural(bad, "transfer") + " failed"
	case down == 0:
		title = Plural(up, "file") + " uploaded"
	case up == 0:
		title = Plural(down, "file") + " downloaded"
	}
	if bad > 0 && up+down > 0 {
		body = fmt.Sprintf("%d couldn’t be transferred. Open Transfers to retry.", bad)
	}
	return title, body, true
}

func Plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
