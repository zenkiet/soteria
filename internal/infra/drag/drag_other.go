//go:build !darwin && !windows

package drag

import (
	"errors"
	"unsafe"

	"soteria/internal/domain"
)

func Start(unsafe.Pointer, []domain.Entry, func() domain.Drive) error {
	return errors.New("drag out is not supported on this platform")
}
