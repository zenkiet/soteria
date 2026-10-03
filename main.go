// Soteria is the composition root: adapters are built, handed to the use cases and run by Wails.
package main

import (
	"embed"
	"log/slog"
	"os"
	"path/filepath"

	"soteria/internal/app"
	"soteria/internal/infra/mount"
	"soteria/internal/infra/store"
	"soteria/internal/infra/wails"
	"soteria/internal/infra/webdav"
)

//go:embed all:frontend/dist
var assets embed.FS

var version string

func main() {
	sess := &app.Session{}
	ev := wails.Events{}
	idx := app.NewIndex(sess, ev)
	tr := app.NewTransfers(sess, idx, ev)
	trash := &app.Trash{S: sess, Index: idx}
	mnt := mount.New(sess.Client, idx.ReindexLater)
	mnt.Adopt = tr.Adopt
	sess.OnConnect = func(c *webdav.Client) {
		idx.Restore()
		idx.Reindex()
		go trash.Sweep(c)
	}
	sess.OnDisconnect = func() {
		_ = mnt.Unmount()
		idx.Clear()
	}
	logger := store.Log()
	// Before 0.6 thumbnails lived in the config folder.
	_ = os.RemoveAll(filepath.Join(store.Dir(), "thumbs"))
	svc := &wails.App{
		S: sess, F: &app.Files{S: sess, Index: idx}, T: tr, Tr: trash, I: idx,
		Th: &app.Thumbs{S: sess, Dir: store.ThumbDir()}, Bg: app.NewBackground(tr), M: mnt, Version: version,
	}
	if err := wails.Run(svc, assets, logger); err != nil {
		slog.Error("soteria stopped", "err", err)
		os.Exit(1)
	}
}
