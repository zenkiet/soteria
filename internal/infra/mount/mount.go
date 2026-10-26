// Package mount shows the server as a drive: mount_webdav behind a loopback proxy that adds the login on macOS, WinFsp on Windows.
package mount

import (
	"sync"

	"soteria/internal/domain"
	"soteria/internal/infra/webdav"
)

// Index is the search index, told what the drive changes.
type Index interface {
	Put(es ...domain.Entry)
	Remove(p string)
	Move(from, to string, keep bool)
}

type Mounter struct {
	client func() (*webdav.Client, error)
	index  Index
	// Adopt takes over a staged write whose PUT failed.
	Adopt func(local, remote string, size int64)
	mu    sync.Mutex
	// busy serialises Mount and Unmount so a second tray click can't start a second mount.
	busy sync.Mutex
}

func New(client func() (*webdav.Client, error), index Index) *Mounter {
	return &Mounter{client: client, index: index}
}
