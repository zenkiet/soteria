//go:build windows

package wails

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"

	"soteria/internal/domain"
	"soteria/internal/infra/drag"
)

// dragOut hands the Shell real paths on the mounted drive and lets it build the data object. That
// needs the drive connected: WebView2 owns the webview's own drag, so there is no way to stream
// the bytes out of the app itself.
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
