package wails

import (
	"io/fs"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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
	a.rt = time.AfterFunc(math.MaxInt64, a.refresh)
	a.T.OnChange = func(status, kind string) {
		a.refreshLater()
		if status != "queued" {
			a.finished(status, kind)
		}
	}
	drag.OnWrite, drag.OnEnded = a.T.DragWrite, func() { Events{}.Emit("dragend", true) }

	wapp := application.New(application.Options{
		Name:        "Soteria",
		Description: "WebDAV client",
		Logger:      logger,
		Services:    []application.Service{application.NewService(a), application.NewService(a.Dock), application.NewService(a.Notes)},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: singleInstanceID,
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				// Not win.Restore(): it also unmaximises.
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
		Title:              "Soteria",
		Width:              1280,
		Height:             800,
		EnableFileDrop:     true,
		ZoomControlEnabled: true,
		// Launch at login passes this on Windows, where the tray can stand in for the window.
		Hidden: slices.Contains(os.Args[1:], "--hidden"),
		Mac: application.MacWindow{
			TitleBar: application.MacTitleBarHiddenInset,
			WebviewPreferences: application.MacWebviewPreferences{
				AllowsBackForwardNavigationGestures: application.Enabled,
			},
		},
		BackgroundColour: application.NewRGB(246, 245, 241),
		URL:              "/",
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		opts.MinWidth = 1024
		opts.MinHeight = 500
	}
	// Deliberately no saved position: a spot on a lost monitor strands the window off-screen.
	a.state = store.LoadWindow()
	if a.state.W > 0 {
		opts.Width, opts.Height = a.state.W, a.state.H
	}
	if c := a.state.Background; c != [3]uint8{} {
		opts.BackgroundColour = application.NewRGB(c[0], c[1], c[2])
	}
	// The default menu's Reload wipes the page's state, and Help → Learn More navigates this window to wails.io.
	if runtime.GOOS == "darwin" {
		menu := application.NewMenu()
		menu.AddRole(application.AppMenu)
		menu.AddRole(application.FileMenu)
		menu.AddRole(application.EditMenu)
		view := menu.AddSubmenu("View")
		view.AddRole(application.ResetZoom)
		view.AddRole(application.ZoomIn)
		view.AddRole(application.ZoomOut)
		view.AddSeparator()
		view.AddRole(application.ToggleFullscreen)
		menu.AddRole(application.WindowMenu)
		wapp.Menu.Set(menu)
	}
	win := wapp.Window.NewWithOptions(opts)
	a.win, a.hidden = win, opts.Hidden
	a.trackWindow(win)
	win.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) { a.fit() })
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
		// A drag out of Soteria on Windows lands back here as drive paths; uploading them copies the server onto itself.
		if drv := a.M.Drive(); drv.Mounted && slices.ContainsFunc(d.Files, func(f string) bool {
			rel, err := filepath.Rel(drv.Path, f)
			return err == nil && filepath.IsLocal(rel)
		}) {
			return
		}
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
func onScreen(screens []*application.Screen, b application.Rect) bool {
	top := application.Point{X: b.X + b.Width/2, Y: b.Y + 20}
	return len(screens) == 0 || slices.ContainsFunc(screens, func(s *application.Screen) bool { return s.Bounds.Contains(top) })
}

// A size saved on a bigger monitor, or the default on a scaled laptop, would overflow the screen.
func (a *App) fit() {
	s, err := a.win.GetScreen()
	if err != nil || s == nil || a.win.IsMaximised() || a.win.IsFullscreen() {
		return
	}
	b, w := a.win.Bounds(), s.WorkArea
	if b.Width > w.Width || b.Height > w.Height {
		a.win.SetSize(min(b.Width, w.Width), min(b.Height, w.Height))
		a.win.Center()
	}
}

// One timer that resize events only push back: Wails runs each event listener in its own goroutine.
func (a *App) trackWindow(w *application.WebviewWindow) {
	save := time.AfterFunc(math.MaxInt64, func() {
		if !w.IsMaximised() {
			width, height := w.Size()
			a.stateMu.Lock()
			a.state.W, a.state.H = width, height
			store.SaveWindow(a.state)
			a.stateMu.Unlock()
		}
	})
	w.OnWindowEvent(events.Common.WindowDidResize, func(*application.WindowEvent) { save.Reset(300 * time.Millisecond) })
}
