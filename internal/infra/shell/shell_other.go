//go:build !darwin && !windows

package shell

import "errors"

var errUnsupported = errors.New("not available on this platform")

func Open(string) error   { return errUnsupported }
func Reveal(string) error { return errUnsupported }
