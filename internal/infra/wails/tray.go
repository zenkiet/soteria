package wails

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"runtime"
	"slices"
	"strconv"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
	"github.com/wailsapp/wails/v3/pkg/updater"

	"soteria/internal/app"
	"soteria/internal/domain"
	"soteria/internal/infra/awake"
	"soteria/internal/infra/shell"
	"soteria/internal/infra/store"
)

func where() string {
	if runtime.GOOS == "windows" {
		return "File Explorer"
	}
	return "Finder"
}

// SetBackground mirrors the two Settings switches; the frontend calls it on start and on change.
func (a *App) SetBackground(on, notify bool) {
	a.Bg.Set(on, notify)
	a.mu.Lock()
	create, destroy := on && a.tray == nil, !on && a.tray != nil
	if create {
		a.tray = application.Get().SystemTray.New()
		if runtime.GOOS == "darwin" {
			a.tray.SetTemplateIcon(glyph(44, color.Black))
		} else {
			a.tray.SetIcon(glyph(32, color.Black)).SetDarkModeIcon(glyph(32, color.White))
		}
		if runtime.GOOS == "windows" {
			a.tray.OnDoubleClick(func() { a.show("") })
		}
	}
	if destroy {
		a.tray.Destroy()
		a.tray = nil
	}
	a.mu.Unlock()
	if create {
		a.refresh()
	}
}

// refreshLater coalesces bursts: a folder upload enqueues hundreds of transfers at once.
func (a *App) refreshLater() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.rt != nil {
		a.rt.Stop()
	}
	a.rt = time.AfterFunc(300*time.Millisecond, a.refresh)
}

func (a *App) refresh() {
	n := a.T.Running()
	awake.Hold(n > 0)
	a.mu.Lock()
	tray := a.tray
	a.mu.Unlock()
	if a.Dock != nil {
		if n > 0 {
			_ = a.Dock.SetBadge(strconv.Itoa(n))
		} else {
			_ = a.Dock.RemoveBadge()
		}
	}
	if tray != nil {
		tray.SetMenu(a.trayMenu(n))
	}
}

func (a *App) trayMenu(n int) *application.Menu {
	cur := a.S.Current()
	m := application.NewMenu()
	head, xfer, quit := "Not signed in", "No transfers", "Quit Soteria"
	if cur != nil {
		head = cur.Username + " · " + cur.Name
	}
	if n > 0 {
		xfer = app.Plural(n, "transfer") + " running"
	}
	if runtime.GOOS == "windows" {
		quit = "Exit"
	}
	m.Add(head).SetEnabled(false)
	m.AddSeparator()
	m.Add("Open Soteria").OnClick(func(*application.Context) { a.show("") })
	m.AddSeparator()
	m.Add(xfer).SetEnabled(false)
	switch application.Get().Updater.State() {
	case updater.StateReady:
		m.Add("Restart to update · " + a.UpdateStatus().Version).OnClick(func(*application.Context) { _ = a.RestartToUpdate() })
	case updater.StateAvailable:
		m.Add("Update available · " + a.UpdateStatus().Version).OnClick(func(*application.Context) { a.show("") })
	case updater.StateIdle, updater.StateUpToDate, updater.StateError:
		m.Add("Check for Updates").OnClick(func(*application.Context) { a.checkNow() })
	}
	m.Add("Open Transfers").OnClick(func(*application.Context) { a.show("/transfers") })
	m.AddSeparator()
	if d := a.M.Drive(); d.Mounted {
		m.Add("Show in " + where()).OnClick(func(*application.Context) { _ = shell.Open(d.Path) })
		m.Add("Disconnect Drive").OnClick(func(*application.Context) { _ = a.M.Unmount(); a.refresh() })
	} else if cur != nil {
		m.Add("Connect Drive").OnClick(func(*application.Context) { _, _ = a.M.Mount(); a.refresh() })
	}
	m.AddSeparator()
	m.Add(quit).OnClick(func(*application.Context) { application.Get().Quit() })
	return m
}

func (a *App) show(path string) {
	if a.win == nil {
		return
	}
	go func() {
		// A monitor unplugged while the window hid in the tray leaves it off-screen.
		x, y := a.win.Position()
		w, _ := a.win.Size()
		if !onScreen(application.Get().Screen.GetAll(), x, y, w) {
			a.win.Center()
		}
		a.Dock.ShowAppIcon()
		a.win.Show()
		a.win.Focus()
		// Windows may refuse SetForegroundWindow from an unfocused process; pulsing topmost raises it anyway.
		a.win.SetAlwaysOnTop(true)
		a.win.SetAlwaysOnTop(false)
		if path != "" {
			Events{}.Emit("nav", path)
		}
	}()
}

// Show blocks on both platforms; the OnClick fallback covers a non-blocking dialog too.
func (a *App) shouldQuit() bool {
	if a.Bg.CanQuit() {
		return true
	}
	n := a.T.Running()
	verb := "are"
	if n == 1 {
		verb = "is"
	}
	d := application.Get().Dialog.Question().SetTitle("Quit Soteria?").
		SetMessage(fmt.Sprintf("%s %s still running and will be cancelled. The network drive will be disconnected.", app.Plural(n, "transfer"), verb))
	d.AddButton("Cancel").SetAsCancel()
	d.AddButton("Quit anyway").OnClick(func() { a.Bg.ForceQuit(); application.Get().Quit() })
	d.Show()
	return a.Bg.CanQuit()
}

// The drained-queue notice gets one button: Retry while anything failed, else Show in Finder for the newest download.
func (a *App) finished(status, kind string) {
	title, body, ok := a.Bg.Finished(status, kind)
	if !ok || a.win == nil || (a.win.IsVisible() && a.win.IsFocused()) {
		return
	}
	n := notifications.NotificationOptions{Title: title, Body: body}
	ts := a.T.List()
	if slices.ContainsFunc(ts, func(t domain.Transfer) bool { return t.Status == "error" }) {
		a.post(n, notifications.NotificationAction{ID: "retry", Title: "Retry"})
		return
	}
	for _, t := range slices.Backward(ts) {
		if t.Kind == "download" && t.Status == "done" {
			n.Data = map[string]any{"path": t.Local}
			a.post(n, notifications.NotificationAction{ID: "reveal", Title: "Show in " + where()})
			return
		}
	}
	a.post(n)
}

func (a *App) respond(r notifications.NotificationResult) {
	switch r.Response.ActionIdentifier {
	case "retry":
		for _, t := range a.T.List() {
			if t.Status == "error" {
				_ = a.T.Retry(t.ID)
			}
		}
	case "reveal":
		if p, ok := r.Response.UserInfo["path"].(string); ok {
			_ = shell.Reveal(p)
		}
	default:
		a.show("")
	}
}

func (a *App) send(title, body string) {
	a.post(notifications.NotificationOptions{Title: title, Body: body})
}

func (a *App) post(n notifications.NotificationOptions, actions ...notifications.NotificationAction) {
	if a.Notes == nil {
		return
	}
	if ok, _ := a.Notes.CheckNotificationAuthorization(); !ok {
		if ok, _ = a.Notes.RequestNotificationAuthorization(); !ok {
			return
		}
	}
	n.ID = rand.Text()
	if len(actions) == 0 {
		_ = a.Notes.SendNotification(n)
		return
	}
	n.CategoryID = actions[0].ID
	_ = a.Notes.RegisterNotificationCategory(notifications.NotificationCategory{ID: n.CategoryID, Actions: actions})
	_ = a.Notes.SendNotificationWithActions(n)
}

// hint says once that closing the window didn't quit.
func (a *App) hint() {
	if a.state.Hinted {
		return
	}
	a.state.Hinted = true
	store.SaveWindow(a.state)
	bar := "menu bar"
	if runtime.GOOS == "windows" {
		bar = "system tray"
	}
	a.send("Soteria is still running", "Find it in the "+bar+". Change this in Settings.")
}

// glyph draws the Bo mark in code, so the tray needs no asset pipeline.
func glyph(size int, c color.Color) []byte {
	f := float64(size)
	// box is the signed distance to the rounded body.
	box := func(x, y float64) float64 {
		dx, dy := math.Abs(x-0.5)-0.33, math.Abs(y-0.55)-0.30
		return math.Hypot(math.Max(dx, 0), math.Max(dy, 0)) + math.Min(math.Max(dx, dy), 0) - 0.09
	}
	eye := func(x, y, cx float64) bool { return math.Hypot((x-cx)/0.055, (y-0.66)/0.065) < 1 }
	ink := func(x, y float64) bool {
		return math.Abs(box(x, y)) < 0.04 || math.Hypot(x-0.29, y-0.42) < 0.10 || math.Hypot(x-0.71, y-0.42) < 0.10 || eye(x, y, 0.39) || eye(x, y, 0.61)
	}
	r, g, b, _ := c.RGBA()
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for py := range size {
		for px := range size {
			n := 0
			for s := range 16 {
				if ink((float64(px)+(float64(s%4)+0.5)/4)/f, (float64(py)+(float64(s/4)+0.5)/4)/f) {
					n++
				}
			}
			img.SetNRGBA(px, py, color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(n * 255 / 16)})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
