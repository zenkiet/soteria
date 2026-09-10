//go:build !darwin && !windows

package drag

import (
	"errors"
	"unsafe"

	"soteria/internal/domain"
)

var (
	OnWrite func(remote, dest string) error
	OnEnded func()
)

func Start(unsafe.Pointer, []domain.Entry) error {
	return errors.New("drag out is not supported on this platform")
}
