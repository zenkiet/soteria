package wails

import (
	"io/fs"
	"os"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/dock"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"soteria/internal/infra/drag"
	"soteria/internal/infra/store"
)

// notifyFirstInstance derives the mutex and message-window names from this too.
const singleInstanceID = "dev.zenkiet.soteria"

// Run wires the adapter to the use cases, builds the Wails app and window, and blocks until quit.
func Run(a *App, assets fs.FS) error {
	notifyFirstInstance() // may exit: hands this launch to the running instance
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
			UniqueID: singleInstanceID,
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				if win != nil {
					win.Restore()
				}
				a.show("") // not win.Show(): show() also rescues an off-screen window
				for _, arg := range data.Args {
					a.openLink(arg)
				}
			},
		},
		Assets:     application.AssetOptions{Handler: application.AssetFileServerFS(assets), Middleware: a.middleware},
		OnShutdown: func() { _ = a.M.Unmount() },
		ShouldQuit: a.shouldQuit,
	})
	a.initUpdater(wapp)
	// Deep links: macOS delivers the launch URL as an Apple Event, Windows as an argv entry
	// (relaunches land in OnSecondInstanceLaunch above). openLink ignores anything else.
	wapp.Event.OnApplicationEvent(events.Common.ApplicationLaunchedWithUrl, func(e *application.ApplicationEvent) {
		a.openLink(e.Context().URL())
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
	// Window geometry survives relaunches, unless it points at a monitor that is gone.
	a.state = store.LoadWindow()
	if a.state.W > 0 && onScreen(wapp.Screen.GetAll(), a.state.X, a.state.Y, a.state.W) {
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
			a.Dock.HideAppIcon()
			a.hint()
		}
	})
	a.Notes.OnNotificationResponse(func(notifications.NotificationResult) { a.show("") })
	win.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		wapp.Event.Emit("dropped", e.Context().DroppedFiles())
	})
	for _, arg := range os.Args[1:] { // Windows cold start passes the URL via argv
		a.openLink(arg)
	}
	return wapp.Run()
}

// onScreen reports whether the window's drag strip would be grabbable on a current
// display. Screens can be empty before the platform reports them: fail open.
func onScreen(screens []*application.Screen, x, y, w int) bool {
	if len(screens) == 0 {
		return true
	}
	cx, cy := x+w/2, y+20
	for _, s := range screens {
		b := s.Bounds
		if cx >= b.X && cx < b.X+b.Width && cy >= b.Y && cy < b.Y+b.Height {
			return true
		}
	}
	return false
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
