package wails

import (
	"errors"
	"io"
	"net/http"

	"soteria/internal/app"
)

// middleware serves the two app-only endpoints the webview loads directly: /preview streams a file, /thumb a thumbnail.
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

func (a *App) preview(w http.ResponseWriter, r *http.Request) {
	resp, err := a.F.Open(r.Context(), r.URL.Query().Get("path"), r.Header.Get("Range"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for _, k := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "ETag", "Last-Modified"} {
		if v := resp.Header.Get(k); v != "" {
			w.Header().Set(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (a *App) thumb(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	w.Header().Set("Cache-Control", "max-age=31536000, immutable")
	file, err := a.Th.File(r.Context(), q.Get("path"), q.Get("v"))
	switch {
	case errors.Is(err, app.ErrUnsupported):
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
	case err != nil:
		http.Error(w, err.Error(), http.StatusBadGateway)
	default:
		http.ServeFile(w, r, file)
	}
}
