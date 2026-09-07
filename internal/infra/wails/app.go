// Package wails is the inbound adapter: the one Wails service the frontend calls, its events, tray, dock badge,
// notifications and window state. Every exported method on App is a binding, so helpers stay unexported.
package wails

import (
	"errors"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/dock"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"soteria/internal/app"
	"soteria/internal/domain"
	"soteria/internal/infra/drag"
	"soteria/internal/infra/mount"
	"soteria/internal/infra/shell"
	"soteria/internal/infra/store"
)

type App struct {
	S  *app.Session
	F  *app.Files
	T  *app.Transfers
	Tr *app.Trash
	I  *app.Index
	Th *app.Thumbs
	Bg *app.Background
	M  *mount.Mounter

	Dock  *dock.DockService
	Notes *notifications.NotificationService
	win   *application.WebviewWindow
	state store.WindowState
	mu    sync.Mutex
	tray  *application.SystemTray
	rt    *time.Timer
}

// Events sends to the webview; app packages only see the interface.
type Events struct{}

func (Events) Emit(name string, data any) {
	if a := application.Get(); a != nil {
		a.Event.Emit(name, data)
	}
}

func init() {
	application.RegisterEvent[domain.Transfer]("transfer")
	application.RegisterEvent[[]string]("dropped")
	application.RegisterEvent[bool]("dragend")
	application.RegisterEvent[domain.IndexStatus]("index")
	application.RegisterEvent[string]("nav")
}

// session
func (a *App) Servers() ([]domain.Server, error)                          { return a.S.Servers() }
func (a *App) Current() *domain.Server                                    { return a.S.Current() }
func (a *App) Connect(s domain.Server, pw string) (*domain.Server, error) { return a.S.Connect(s, pw) }

func (a *App) ConnectSaved(id string) (*domain.Server, error) { return a.S.ConnectSaved(id) }
func (a *App) SignOut()                                       { a.S.SignOut() }
func (a *App) Disconnect()                                    { a.S.Disconnect() }
func (a *App) Forget(id string) error                         { return a.S.Forget(id) }

// files
func (a *App) List(p string) ([]domain.Entry, error) { return a.F.List(p) }
func (a *App) Quota() (domain.Quota, error)          { return a.F.Quota() }
func (a *App) Mkdir(p string) error                  { return a.F.Mkdir(p) }
func (a *App) Move(from, to string) error            { return a.F.Move(from, to) }
func (a *App) Copy(from, to string) error            { return a.F.Copy(from, to) }
func (a *App) Ping() error                           { return a.F.Ping() }
func (a *App) Link(p string) (string, error)         { return a.F.Link(p) }

// transfers
func (a *App) Transfers() []domain.Transfer { return a.T.List() }
func (a *App) Cancel(id string)             { a.T.Cancel(id) }
func (a *App) CancelGroup(group string)     { a.T.CancelGroup(group) }
func (a *App) Retry(id string) error        { return a.T.Retry(id) }
func (a *App) Clear()                       { a.T.Clear() }
func (a *App) Upload(locals []string, dir string) ([]domain.Conflict, error) {
	return a.T.Upload(locals, dir)
}

func (a *App) UploadAs(local, remote, mode string) error { return a.T.UploadAs(local, remote, mode) }

func (a *App) Download(remote string, size int64, dest string) (string, error) {
	return a.T.Download(remote, size, dest)
}
func (a *App) DownloadDir(remote, dest string) error { return a.T.DownloadDir(remote, dest) }
func (a *App) Reveal(local string) error             { return shell.Reveal(local) }

func (a *App) PickUploads(dir string) ([]domain.Conflict, error) {
	files, err := application.Get().Dialog.OpenFile().SetTitle("Upload to " + dir).CanChooseDirectories(true).PromptForMultipleSelection()
	if err != nil {
		return nil, err
	}
	return a.T.Upload(files, dir)
}

func (a *App) PickFolder() (string, error) {
	return application.Get().Dialog.OpenFile().CanChooseDirectories(true).CanChooseFiles(false).PromptForSingleSelection()
}

// trash
func (a *App) Trash(p string) (string, error)         { return a.Tr.Trash(p) }
func (a *App) ListTrash() ([]domain.TrashItem, error) { return a.Tr.List() }
func (a *App) Restore(p string) error                 { return a.Tr.Restore(p) }
func (a *App) Purge(p string) error                   { return a.Tr.Purge(p) }
func (a *App) EmptyTrash() error                      { return a.Tr.Empty() }

// search
func (a *App) Reindex()                    { a.I.Reindex() }
func (a *App) Indexed() domain.IndexStatus { return a.I.Status() }
func (a *App) Recent(n int) []domain.Entry { return a.I.Recent(n) }
func (a *App) Search(q string) []domain.Entry {
	return a.I.Search(q)
}

// drive
func (a *App) Drive() domain.Drive          { return a.M.Drive() }
func (a *App) Mount() (domain.Drive, error) { return a.M.Mount() }
func (a *App) Unmount() error               { return a.M.Unmount() }
func (a *App) InstallDriver() error         { return a.M.InstallDriver() }
func (a *App) Open(p string) error          { return shell.Open(p) }

func (a *App) LogPath() string { return store.LogPath() }

// DragOut starts a native drag of remote files; Finder asks for each file on drop and it is downloaded there.
func (a *App) DragOut(entries []domain.Entry) error {
	if len(entries) == 0 {
		return nil
	}
	win, ok := application.Get().Window.Current().(*application.WebviewWindow)
	if !ok {
		return errors.New("no window")
	}
	a.T.SetDragging(entries)
	var err error
	application.InvokeSync(func() { err = drag.Start(win.NativeWindow(), entries) })
	return err
}
