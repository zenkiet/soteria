package domain

import (
	"errors"
	"io/fs"
)

var ErrOffline = errors.New("can't reach the server")

// DavError is a WebDAV status the user can act on; Text is already a sentence.
type DavError struct {
	Code int
	Text string
}

func (e *DavError) Error() string { return e.Text }

// Retryable: network trouble (stall, reset, unreachable) is worth another try; server answers and local file errors are not.
func Retryable(err error) bool {
	var de *DavError
	var pe *fs.PathError
	return err != nil && !errors.As(err, &de) && !errors.As(err, &pe)
}
