package mount

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"soteria/internal/domain"
	"soteria/internal/infra/webdav"
)

func TestLoopbackProxy(t *testing.T) {
	var gotPath, gotDest, gotAuth string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotDest, gotAuth = r.URL.Path, r.Header.Get("Destination"), r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
	}))
	defer up.Close()
	c, _ := webdav.New(domain.Server{URL: up.URL + "/dav", Username: "u"}, "p")
	m := New(func() (*webdav.Client, error) { return c, nil }, nil)
	if err := m.serveLoopback(); err != nil {
		t.Fatal(err)
	}
	local := "http://" + ln.Addr().String()
	req, _ := http.NewRequest("MOVE", local+"/a", nil)
	req.Header.Set("Destination", local+"/b")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("proxy: %s", resp.Status)
	}
	if gotPath != "/dav/a" || gotDest != up.URL+"/dav/b" || !strings.HasPrefix(gotAuth, "Basic ") {
		t.Fatalf("upstream saw path=%q dest=%q auth=%q", gotPath, gotDest, gotAuth)
	}
	req, _ = http.NewRequest("GET", local+"/a", nil)
	req.Header.Set("Origin", "https://evil.example")
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("a browser request must be refused: %v %v", err, resp)
	} else {
		resp.Body.Close()
	}
}
