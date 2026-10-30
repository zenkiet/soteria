package wails

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"runtime"
	"strconv"
	"strings"

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
	rng := r.Header.Get("Range")
	// WebView2 gets the body only once this handler returns, so a video's "bytes=0-" would buffer the whole file in RAM.
	if runtime.GOOS == "windows" {
		rng = clampRange(rng)
	}
	resp, err := a.F.Open(r.Context(), p, rng)
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
		// Wails answers a 304 with a 500 on Windows (a 304 hangs WebView2); the URL is versioned anyway.
		r.Header.Del("If-Modified-Since")
		http.ServeFile(w, r, file)
	}
}

const previewChunk = 8 << 20

// clampRange caps a single byte range at previewChunk; media players ask for the rest as they play.
func clampRange(h string) string {
	from, to, ok := strings.Cut(strings.TrimPrefix(h, "bytes="), "-")
	start, err := strconv.ParseInt(from, 10, 64)
	if !ok || err != nil || !strings.HasPrefix(h, "bytes=") || strings.Contains(h, ",") {
		return h
	}
	if end, err := strconv.ParseInt(to, 10, 64); err == nil && end-start < previewChunk {
		return h
	}
	return fmt.Sprintf("bytes=%d-%d", start, start+previewChunk-1)
}
