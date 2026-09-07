package wails

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"

	"soteria/internal/domain"
)

// Self-update rides on the Wails updater in headless mode: the frontend draws the dialog and calls these bindings.
func (a *App) initUpdater(wapp *application.App) {
	if a.Version == "" {
		return
	}
	gh, _ := github.New(github.Config{Repository: "blogic-kietle/BStorage", ChecksumAsset: "SHA256SUMS.txt", AssetMatcher: matchAsset})
	_ = wapp.Updater.Init(updater.Config{CurrentVersion: a.Version, Providers: []updater.Provider{gh}, Window: updater.WindowNone})
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	if i := strings.Index(exe, ".app/"); runtime.GOOS == "darwin" && i >= 0 {
		dir = filepath.Dir(exe[:i+4])
	}
	a.blocked = !writableDir(dir)
}

// matchAsset picks Soteria-<v>-<GOOS>-<GOARCH>.zip; the DMG and Setup.exe are for people.
func matchAsset(req updater.CheckRequest, assets []github.ReleaseAsset) int {
	suffix := "-" + req.Platform + "-" + req.Arch + ".zip"
	for i, x := range assets {
		if strings.HasSuffix(x.Name, suffix) {
			return i
		}
	}
	return -1
}

// writableDir is false where the helper could not swap the app: a mounted DMG, an admin-only Program Files.
func writableDir(dir string) bool {
	f, err := os.CreateTemp(dir, ".soteria-*")
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(f.Name())
	return true
}

func (a *App) UpdateStatus() domain.Update {
	a.mu.Lock()
	rel := a.rel
	a.mu.Unlock()
	s := domain.Update{Current: a.Version, State: string(application.Get().Updater.State()), Blocked: a.blocked}
	if rel != nil {
		s.Version, s.Notes, s.Size, s.Date = rel.Version, rel.Notes, rel.Artifact.Size, rel.PublishedAt
		s.URL, _ = rel.Metadata["github.release.htmlURL"].(string)
	}
	return s
}

func (a *App) CheckUpdate() error {
	rel, err := application.Get().Updater.Check(context.Background())
	a.mu.Lock()
	a.rel = rel
	a.mu.Unlock()
	a.refresh()
	return err
}

// InstallUpdate downloads and verifies in the background; progress arrives on the wails:updater:* events.
func (a *App) InstallUpdate() {
	go func() {
		if application.Get().Updater.DownloadAndInstall(context.Background()) != nil {
			return
		}
		a.refresh()
		if a.win != nil && (!a.win.IsVisible() || !a.win.IsFocused()) {
			a.send("Soteria "+a.UpdateStatus().Version+" is ready", "Restart to finish updating.")
		}
	}()
}

func (a *App) RestartToUpdate() error {
	a.Bg.ForceQuit()
	return application.Get().Updater.Restart(context.Background())
}
