// Soteria: composition root. Adapters are created, handed to the use cases, and the Wails adapter runs the app.
package main

import (
	"embed"
	"log"

	"soteria/internal/app"
	"soteria/internal/infra/mount"
	"soteria/internal/infra/store"
	"soteria/internal/infra/wails"
	"soteria/internal/infra/webdav"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	sess := &app.Session{}
	ev := wails.Events{}
	idx := app.NewIndex(sess, ev)
	tr := app.NewTransfers(sess, idx, ev)
	trash := &app.Trash{S: sess, Index: idx}
	mnt := mount.New(sess.Client, idx.ReindexLater)
	sess.OnConnect = func(c *webdav.Client) {
		idx.Clear()
		idx.Reindex()
		go trash.Sweep(c)
	}
	sess.OnDisconnect = func() {
		_ = mnt.Unmount()
		idx.Clear()
	}
	svc := &wails.App{
		S: sess, F: &app.Files{S: sess, Index: idx}, T: tr, Tr: trash, I: idx,
		Th: &app.Thumbs{S: sess, Dir: store.ThumbDir()}, Bg: app.NewBackground(tr), M: mnt,
	}
	if err := wails.Run(svc, assets); err != nil {
		log.Fatal(err)
	}
}
