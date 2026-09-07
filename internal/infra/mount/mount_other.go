//go:build !darwin && !windows

package mount

import (
	"errors"

	"soteria/internal/domain"
)

var errNoDrive = errors.New("network drive is not available on this platform yet")

func (m *Mounter) Drive() domain.Drive          { return domain.Drive{} }
func (m *Mounter) Mount() (domain.Drive, error) { return domain.Drive{}, errNoDrive }
func (m *Mounter) Unmount() error               { return nil }
func (m *Mounter) InstallDriver() error         { return errNoDrive }
