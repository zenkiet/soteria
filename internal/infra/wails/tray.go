package wails

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"runtime"
	"strconv"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
	"github.com/wailsapp/wails/v3/pkg/updater"

	"soteria/internal/app"
	"soteria/internal/infra/shell"
	"soteria/internal/infra/store"
)

// Background mode: closing hides the window, the app lives in the menu bar / tray,
// the Dock badge counts running transfers, one notification per finished queue.

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
	if application.Get().Updater.State() == updater.StateReady {
		m.Add("Restart to update · " + a.UpdateStatus().Version).OnClick(func(*application.Context) { _ = a.RestartToUpdate() })
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
	a.win.Show()
	a.win.Focus()
	if path != "" {
		Events{}.Emit("nav", path)
	}
}

// shouldQuit asks before cancelling running transfers. Show blocks on both platforms; the
// OnClick fallback covers a non-blocking dialog too.
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

// finished notifies once per drained queue, only when the window isn't in front.
func (a *App) finished(status, kind string) {
	title, body, ok := a.Bg.Finished(status, kind)
	if !ok || a.win == nil || (a.win.IsVisible() && a.win.IsFocused()) {
		return
	}
	a.send(title, body)
}

func (a *App) send(title, body string) {
	if a.Notes == nil {
		return
	}
	if ok, _ := a.Notes.CheckNotificationAuthorization(); !ok {
		if ok, _ = a.Notes.RequestNotificationAuthorization(); !ok {
			return
		}
	}
	_ = a.Notes.SendNotification(notifications.NotificationOptions{ID: strconv.FormatInt(time.Now().UnixNano(), 36), Title: title, Body: body})
}

// hint tells the user once that closing didn't quit.
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

// glyph draws the monochrome Bo mark (folder outline, ears, eyes) as a PNG: no asset pipeline needed.
func glyph(size int, c color.Color) []byte {
	f := float64(size)
	box := func(x, y float64) float64 { // signed distance to the rounded body
		dx, dy := math.Abs(x-0.5)-0.33, math.Abs(y-0.55)-0.30
		return math.Hypot(math.Max(dx, 0), math.Max(dy, 0)) + math.Min(math.Max(dx, dy), 0) - 0.09
	}
	eye := func(x, y, cx float64) bool { return math.Hypot((x-cx)/0.055, (y-0.66)/0.065) < 1 }
	ink := func(x, y float64) bool {
		return math.Abs(box(x, y)) < 0.04 || math.Hypot(x-0.29, y-0.42) < 0.10 || math.Hypot(x-0.71, y-0.42) < 0.10 || eye(x, y, 0.39) || eye(x, y, 0.61)
	}
	r, g, b, _ := c.RGBA()
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			n := 0
			for s := 0; s < 16; s++ {
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
