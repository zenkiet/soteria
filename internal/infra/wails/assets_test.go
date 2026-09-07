package wails

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"soteria/internal/app"
	"soteria/internal/domain"
	"soteria/internal/infra/webdav"
)

func TestPreviewProxy(t *testing.T) {
	png := []byte("\x89PNG fake body")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dav/Product Photos/a.png" || r.Method != http.MethodGet {
			t.Errorf("unexpected upstream request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Accept-Ranges", "bytes")
		if r.Header.Get("Range") == "bytes=0-3" {
			w.Header().Set("Content-Range", "bytes 0-3/14")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(png[:4])
			return
		}
		_, _ = w.Write(png)
	}))
	defer srv.Close()

	c, _ := webdav.New(domain.Server{URL: srv.URL + "/dav", Username: "pos"}, "x")
	s := &app.Session{}
	s.Use(c, nil)
	a := &App{F: &app.Files{S: s}}
	h := a.middleware(http.NotFoundHandler())

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/preview?path=%2FProduct%20Photos%2Fa.png", nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" || rec.Body.String() != string(png) {
		t.Fatalf("full: %d %q %q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/preview?path=%2FProduct%20Photos%2Fa.png", nil)
	req.Header.Set("Range", "bytes=0-3")
	h.ServeHTTP(rec, req)
	if rec.Code != 206 || rec.Header().Get("Content-Range") != "bytes 0-3/14" || rec.Body.Len() != 4 {
		t.Fatalf("range: %d %q %d", rec.Code, rec.Header().Get("Content-Range"), rec.Body.Len())
	}
}
