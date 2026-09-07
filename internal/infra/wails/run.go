package wails

import (
	"io/fs"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/dock"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"soteria/internal/infra/drag"
	"soteria/internal/infra/store"
)

// Run wires the adapter to the use cases, builds the Wails app and window, and blocks until quit.
func Run(a *App, assets fs.FS) error {
	a.Dock, a.Notes = dock.New(), notifications.New()
	a.T.OnChange = func(status, kind string) {
		a.refreshLater()
		if status != "queued" {
			a.finished(status, kind)
		}
	}
	drag.OnWrite, drag.OnEnded = a.T.DragWrite, func() { Events{}.Emit("dragend", true) }

	var win *application.WebviewWindow
	wapp := application.New(application.Options{
		Name:        "Soteria",
		Description: "WebDAV client",
		Services:    []application.Service{application.NewService(a), application.NewService(a.Dock), application.NewService(a.Notes)},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "dev.zenkiet.soteria",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if win != nil {
					win.Show()
					win.Restore()
					win.Focus()
				}
			},
		},
		Assets:     application.AssetOptions{Handler: application.AssetFileServerFS(assets), Middleware: a.middleware},
		Mac:        application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		OnShutdown: func() { _ = a.M.Unmount() },
		ShouldQuit: a.shouldQuit,
	})

	opts := application.WebviewWindowOptions{
		Title:          "Soteria",
		Frameless:      runtime.GOOS == "windows",
		Width:          1280,
		Height:         800,
		EnableFileDrop: true,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 52,
			TitleBar:                application.MacTitleBarHiddenInset,
			WebviewPreferences: application.MacWebviewPreferences{
				AllowsBackForwardNavigationGestures: application.Enabled,
			},
		},
		BackgroundColour: application.NewRGB(246, 245, 241),
		URL:              "/",
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		opts.MinWidth = 1280
		opts.MinHeight = 600
	}
	// Window geometry survives relaunches. ponytail: no off-screen check; validate against Screens if a user loses the window after unplugging a monitor.
	a.state = store.LoadWindow()
	if a.state.W > 0 {
		opts.X, opts.Y, opts.Width, opts.Height, opts.InitialPosition = a.state.X, a.state.Y, a.state.W, a.state.H, application.WindowXY
	}
	win = wapp.Window.NewWithOptions(opts)
	a.win = win
	a.trackWindow(win)
	// Background mode: the close button hides instead of closing; Quit lives in the tray menu and ⌘Q.
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if a.Bg.Enabled() {
			e.Cancel()
			win.Hide()
			a.hint()
		}
	})
	a.Notes.OnNotificationResponse(func(notifications.NotificationResult) { a.show("") })
	win.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		wapp.Event.Emit("dropped", e.Context().DroppedFiles())
	})
	return wapp.Run()
}

// trackWindow writes the geometry 300 ms after the last move or resize.
func (a *App) trackWindow(w *application.WebviewWindow) {
	var t *time.Timer
	save := func(*application.WindowEvent) {
		if t != nil {
			t.Stop()
		}
		t = time.AfterFunc(300*time.Millisecond, func() {
			if w.IsMaximised() {
				return
			}
			a.state.X, a.state.Y = w.Position()
			a.state.W, a.state.H = w.Size()
			store.SaveWindow(a.state)
		})
	}
	w.OnWindowEvent(events.Common.WindowDidMove, save)
	w.OnWindowEvent(events.Common.WindowDidResize, save)
}
