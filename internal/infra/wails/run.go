package wails

import (
	"io/fs"
	"log/slog"
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
func Run(a *App, assets fs.FS, logger *slog.Logger) error {
	// May exit: hands this launch to the running instance.
	notifyFirstInstance()
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
		Logger:      logger,
		Services:    []application.Service{application.NewService(a), application.NewService(a.Dock), application.NewService(a.Notes)},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: singleInstanceID,
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				if win != nil {
					win.Restore()
				}
				// Not win.Show(): show() also rescues an off-screen window.
				a.show("")
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
	// macOS delivers the launch URL as an Apple Event; Windows passes it in argv (below) or to OnSecondInstanceLaunch.
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
	if runtime.GOOS == "windows" {
		// Mica on Windows 11 (blur on 10) shows through wherever the page leaves its background transparent.
		opts.BackgroundType = application.BackgroundTypeTranslucent
		opts.Windows.BackdropType = application.Mica
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		opts.MinWidth = 1280
		opts.MinHeight = 600
	}
	// Deliberately no saved position: a spot on a lost monitor strands the window off-screen.
	a.state = store.LoadWindow()
	if a.state.W > 0 {
		opts.Width, opts.Height = a.state.W, a.state.H
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
	a.Notes.OnNotificationResponse(a.respond)
	win.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		d := Drop{Files: e.Context().DroppedFiles()}
		if t := e.Context().DropTargetDetails(); t != nil {
			d.Dir = t.Attributes["data-path"]
		}
		wapp.Event.Emit("dropped", d)
	})
	for _, arg := range os.Args[1:] {
		a.openLink(arg)
	}
	return wapp.Run()
}

// Screens can be empty before the platform reports them, so this fails open.
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

func (a *App) trackWindow(w *application.WebviewWindow) {
	var t *time.Timer
	w.OnWindowEvent(events.Common.WindowDidResize, func(*application.WindowEvent) {
		if t != nil {
			t.Stop()
		}
		t = time.AfterFunc(300*time.Millisecond, func() {
			if w.IsMaximised() {
				return
			}
			a.state.W, a.state.H = w.Size()
			store.SaveWindow(a.state)
		})
	})
}
