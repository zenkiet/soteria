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

// Retryable reports whether err is network trouble worth another try; server answers and local file errors are not.
func Retryable(err error) bool {
	_, dav := errors.AsType[*DavError](err)
	_, local := errors.AsType[*fs.PathError](err)
	return err != nil && !dav && !local
}
