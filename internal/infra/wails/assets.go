package wails

import (
	"cmp"
	"errors"
	"io"
	"mime"
	"net/http"
	"path"

	"soteria/internal/app"
)

func (a *App) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/preview":
			a.preview(w, r)
		case "/thumb":
			a.thumb(w, r)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

// This origin holds the Wails bridge: type by extension so a server can't get HTML rendered in the preview iframe.
func (a *App) preview(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	resp, err := a.F.Open(r.Context(), p, r.Header.Get("Range"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for _, k := range []string{"Content-Length", "Content-Range", "Accept-Ranges", "ETag", "Last-Modified"} {
		if v := resp.Header.Get(k); v != "" {
			w.Header().Set(k, v)
		}
	}
	w.Header().Set("Content-Type", cmp.Or(mime.TypeByExtension(path.Ext(p)), "application/octet-stream"))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (a *App) thumb(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	file, err := a.Th.File(r.Context(), q.Get("path"), q.Get("v"))
	switch {
	case errors.Is(err, app.ErrUnsupported):
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
	case err != nil:
		http.Error(w, err.Error(), http.StatusBadGateway)
	default:
		w.Header().Set("Cache-Control", "max-age=31536000, immutable")
		http.ServeFile(w, r, file)
	}
}
