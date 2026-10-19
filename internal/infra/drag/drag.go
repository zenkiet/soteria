// Package drag starts a native drag of remote items out of the window: macOS promises them and OnWrite downloads each on drop, Windows hands Explorer their paths on the mounted drive.
package drag

import "soteria/internal/domain"

var (
	// OnWrite downloads e to dest and must block until done.
	OnWrite func(e domain.Entry, dest string) error
	OnEnded func()
)
