// Package mount shows the server as a drive: mount_webdav behind a loopback proxy that adds the login on macOS, WinFsp on Windows.
package mount

import (
	"sync"

	"soteria/internal/infra/webdav"
)

type Mounter struct {
	client func() (*webdav.Client, error)
	// onChange lets the search index follow what the drive writes.
	onChange func()
	// Adopt takes over a staged write whose PUT failed.
	Adopt func(local, remote string, size int64)
	mu    sync.Mutex
}

func New(client func() (*webdav.Client, error), onChange func()) *Mounter {
	return &Mounter{client: client, onChange: onChange}
}
