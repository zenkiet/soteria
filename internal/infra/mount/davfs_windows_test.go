package mount

import (
	"net/http/httptest"
	"testing"

	"github.com/winfsp/cgofuse/fuse"

	"soteria/internal/domain"
	"soteria/internal/infra/webdav"
	"soteria/internal/infra/webdav/webdavtest"
)

func TestDavFS(t *testing.T) {
	f := webdavtest.New(map[string]bool{"/": true, "/Docs": true}, map[string]string{"/Docs/a.txt": "hello world"})
	srv := httptest.NewServer(f)
	defer srv.Close()
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
	if errc != 0 || fs.Read("/Docs/a.txt", buf, 6, h) != 5 || string(buf) != "world" {
		t.Fatalf("range read: %d %q", errc, buf)
	}
	fs.Release("/Docs/a.txt", h)

	_, h = fs.Create("/Docs/b.txt", fuse.O_WRONLY, 0o644)
	fs.Write("/Docs/b.txt", []byte("new"), 0, h)
	if fs.Getattr("/Docs/b.txt", &st, h) != 0 || st.Size != 3 {
		t.Fatalf("staged getattr: %+v", st)
	}
	if fs.Release("/Docs/b.txt", h) != 0 || f.Files["/Docs/b.txt"] != "new" {
		t.Fatalf("release should PUT: %v", f.Files)
	}
	if fs.Rename("/Docs/b.txt", "/Docs/c.txt") != 0 || f.Files["/Docs/c.txt"] != "new" {
		t.Fatalf("rename: %v", f.Files)
	}
	if fs.Rename("/Docs/c.txt", "/Docs/a.txt") != -fuse.EEXIST {
		t.Fatal("rename onto existing should be EEXIST")
	}
	if fs.Unlink("/Docs/c.txt") != 0 || f.Files["/Docs/c.txt"] != "" {
		t.Fatalf("unlink: %v", f.Files)
	}
	if fs.Mkdir("/New", 0o755) != 0 || !f.Dirs["/New"] {
		t.Fatalf("mkdir: %v", f.Dirs)
	}
	var sf fuse.Statfs_t
	if fs.Statfs("/", &sf) != 0 || sf.Blocks == 0 {
		t.Fatalf("statfs: %+v", sf)
	}
}
