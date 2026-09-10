//go:build windows

package wails

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"

	"soteria/internal/domain"
	"soteria/internal/infra/drag"
)

func (a *App) dragOut(entries []domain.Entry) error {
	var drive domain.Drive
	if a.M != nil {
		drive = a.M.Drive()
	}
	paths, err := windowsDragPaths(drive, entries)
	if err != nil {
		return err
	}
	app := application.Get()
	if app == nil {
		return errors.New("no window")
	}
	win, ok := app.Window.Current().(*application.WebviewWindow)
	if !ok {
		return errors.New("no window")
	}
	application.InvokeSync(func() { err = drag.StartPaths(win.NativeWindow(), paths) })
	return err
}
