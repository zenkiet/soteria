//go:build !windows

package wails

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"

	"soteria/internal/domain"
	"soteria/internal/infra/drag"
)

func (a *App) dragOut(entries []domain.Entry) error {
	win, ok := application.Get().Window.Current().(*application.WebviewWindow)
	if !ok {
		return errors.New("no window")
	}
	a.T.SetDragging(entries)
	var err error
	application.InvokeSync(func() { err = drag.Start(win.NativeWindow(), entries) })
	return err
}
