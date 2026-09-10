package drag

var (
	// OnWrite downloads remote to dest and blocks until done. macOS file promises use it.
	OnWrite func(remote, dest string) error
	// OnEnded reports that the drag finished, so the frontend can drop its dragging state.
	OnEnded func()
)
