package mount

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/winfsp/cgofuse/fuse"

	"soteria/internal/domain"
	"soteria/internal/infra/webdav"
	"soteria/internal/infra/webdav/webdavtest"
)

func TestDavFS(t *testing.T) {
	f := webdavtest.New(map[string]bool{"/": true, "/Docs": true}, map[string]string{"/Docs/a.txt": "hello world"})
	gets := 0
	srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gets++
		}
		f.ServeHTTP(w, r)
	}))
	srv.Start()
	c, _ := webdav.New(domain.Server{URL: srv.URL, Username: "u"}, "p")
	fs := newDavFS(New(func() (*webdav.Client, error) { return c, nil }, nil))
	none := ^uint64(0)

	var st fuse.Stat_t
	if fs.Getattr("/Docs/a.txt", &st, none) != 0 || st.Size != 11 || st.Mode&fuse.S_IFREG == 0 {
		t.Fatalf("getattr: %+v", st)
	}
	if fs.Getattr("/nope", &st, none) != -fuse.ENOENT {
		t.Fatal("missing file should be ENOENT")
	}
	var names []string
	fs.Readdir("/", func(n string, _ *fuse.Stat_t, _ int64) bool { names = append(names, n); return true }, 0, none)
	if len(names) != 3 || names[2] != "Docs" {
		t.Fatalf("readdir: %v", names)
	}

	errc, h := fs.Open("/Docs/a.txt", fuse.O_RDONLY)
	buf := make([]byte, 5)
	if errc != 0 || fs.Read("/Docs/a.txt", buf, 0, h) != 5 || string(buf) != "hello" {
		t.Fatalf("range read: %d %q", errc, buf)
	}
	if fs.Read("/Docs/a.txt", buf, 6, h) != 5 || string(buf) != "world" || gets != 1 {
		t.Fatalf("read-ahead: %q after %d GETs", buf, gets)
	}
	fs.Release("/Docs/a.txt", h)

	_, h = fs.Create("/Docs/b.txt", fuse.O_WRONLY, 0o644)
	fs.Write("/Docs/b.txt", []byte("new"), 0, h)
	if fs.Getattr("/Docs/b.txt", &st, h) != 0 || st.Size != 3 {
		t.Fatalf("staged getattr: %+v", st)
	}
	names = nil
	fs.Readdir("/Docs", func(n string, _ *fuse.Stat_t, _ int64) bool { names = append(names, n); return true }, 0, none)
	if !slices.Contains(names, "b.txt") {
		t.Fatalf("staged file missing from its folder: %v", names)
	}
	_, r := fs.Open("/Docs/b.txt", fuse.O_RDONLY)
	if fs.Read("/Docs/b.txt", buf, 0, r) != 3 || string(buf[:3]) != "new" {
		t.Fatalf("a file not uploaded yet should read from its local copy: %q", buf)
	}
	fs.Release("/Docs/b.txt", r)
	if fs.Rename("/Docs/b.txt", "/Docs/c.txt") != 0 {
		t.Fatal("renaming a file not uploaded yet should retarget it")
	}
	rc := fs.Release("/Docs/c.txt", h)
	if _, ok := f.Files["/Docs/b.txt"]; rc != 0 || f.Files["/Docs/c.txt"] != "new" || ok {
		t.Fatalf("release should PUT under the new name: %v", f.Files)
	}
	if fs.Rename("/Docs/c.txt", "/Docs/a.txt") != 0 || f.Files["/Docs/a.txt"] != "new" {
		t.Fatalf("rename should replace: %v", f.Files)
	}

	_, h = fs.Open("/Docs/a.txt", fuse.O_RDWR)
	fs.Truncate("/Docs/a.txt", 0, h)
	fs.Write("/Docs/a.txt", []byte("x"), 0, h)
	if fs.Release("/Docs/a.txt", h) != 0 || f.Files["/Docs/a.txt"] != "x" || gets != 1 {
		t.Fatalf("overwrite shouldn't download: %q after %d GETs", f.Files["/Docs/a.txt"], gets)
	}
	_, h = fs.Open("/Docs/a.txt", fuse.O_RDWR)
	_, h2 := fs.Open("/Docs/a.txt", fuse.O_RDWR)
	fs.Write("/Docs/a.txt", []byte("y"), 1, h)
	fs.Write("/Docs/a.txt", []byte("z"), 2, h2)
	fs.Release("/Docs/a.txt", h)
	if f.Files["/Docs/a.txt"] != "x" || fs.Release("/Docs/a.txt", h2) != 0 || f.Files["/Docs/a.txt"] != "xyz" || gets != 2 {
		t.Fatalf("writers should share one copy, uploaded by the last close: %q after %d GETs", f.Files["/Docs/a.txt"], gets)
	}

	_, h = fs.Create("/Docs/d.txt", fuse.O_WRONLY, 0o644)
	fs.Write("/Docs/d.txt", []byte("tmp"), 0, h)
	if fs.Unlink("/Docs/d.txt") != 0 || fs.Getattr("/Docs/d.txt", &st, none) != -fuse.ENOENT {
		t.Fatal("unlink of an open file should hide it")
	}
	rc = fs.Release("/Docs/d.txt", h)
	if _, ok := f.Files["/Docs/d.txt"]; rc != 0 || ok {
		t.Fatalf("a file deleted while open shouldn't upload: %v", f.Files)
	}
	if fs.Unlink("/Docs/a.txt") != 0 || f.Files["/Docs/a.txt"] != "" {
		t.Fatalf("unlink: %v", f.Files)
	}
	if fs.Mkdir("/New", 0o755) != 0 || !f.Dirs["/New"] {
		t.Fatalf("mkdir: %v", f.Dirs)
	}
	var sf fuse.Statfs_t
	if fs.Statfs("/", &sf) != 0 || sf.Blocks == 0 {
		t.Fatalf("statfs: %+v", sf)
	}
	if fs.Rename("/Docs", "/New") != -fuse.EEXIST {
		t.Fatal("rename onto a folder must not replace it")
	}
}
