package app

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"soteria/internal/domain"
	"soteria/internal/infra/webdav"
	"soteria/internal/infra/webdav/webdavtest"
)

type noEvents struct{}

func (noEvents) Emit(string, any) {}

func session(t *testing.T, url string) *Session {
	c, err := webdav.New(domain.Server{URL: url, Username: "u"}, "p")
	if err != nil {
		t.Fatal(err)
	}
	s := &Session{}
	s.Use(c, nil)
	return s
}

func transfers(s *Session) *Transfers { return NewTransfers(s, NewIndex(s, noEvents{}), noEvents{}) }

func TestSearch(t *testing.T) {
	idx := NewIndex(&Session{}, noEvents{})
	idx.entries = []domain.Entry{{Name: "Photos", Path: "/Photos", Dir: true}, {Name: "cat.png", Path: "/Photos/cat.png"}, {Name: "notes.txt", Path: "/notes.txt"}}
	if got := idx.Search("CAT"); len(got) != 1 || got[0].Path != "/Photos/cat.png" {
		t.Fatalf("search: %+v", got)
	}
	if got := idx.Search(" "); len(got) != 0 {
		t.Fatalf("blank query should match nothing, got %d", len(got))
	}
	if got := idx.Recent(5); len(got) != 2 {
		t.Fatalf("recent skips folders: %+v", got)
	}
}

func TestUploadConflictsAndFreeName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>/a.png</D:href><D:propstat><D:prop><D:resourcetype/><D:getcontentlength>7</D:getcontentlength></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response></D:multistatus>`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	for _, n := range []string{"a.png", "b.png"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tr := transfers(session(t, srv.URL))
	conflicts, err := tr.Upload([]string{filepath.Join(dir, "a.png"), filepath.Join(dir, "b.png")}, "/")
	if err != nil || len(conflicts) != 1 || conflicts[0].Remote != "/a.png" || conflicts[0].Size != 7 {
		t.Fatalf("conflicts: %v %+v", err, conflicts)
	}
	if ts := tr.List(); len(ts) != 1 || ts[0].Name != "b.png" {
		t.Fatalf("expected only b.png queued, got %+v", ts)
	}
	taken := map[string]bool{"/a.png": true, "/a 2.png": true}
	if got := domain.FreeName("/a.png", func(p string) bool { return taken[p] }); got != "/a 3.png" {
		t.Fatalf("free: %s", got)
	}
}

func TestRetryKeepsID(t *testing.T) {
	s := session(t, "http://127.0.0.1:0")
	c, _ := s.Client()
	tr := transfers(s)
	missing := filepath.Join(t.TempDir(), "missing")
	id := tr.enqueue(domain.Transfer{Kind: "upload", Name: "x", Local: missing, Remote: "/x"}, upload(c, missing, "/x", 0))
	wait := func() domain.Transfer {
		for i := 0; i < 100; i++ {
			if ts := tr.List(); len(ts) == 1 && ts[0].Status == "error" {
				return ts[0]
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("transfer never failed: %+v", tr.List())
		return domain.Transfer{}
	}
	wait()
	if err := tr.Retry(id); err != nil {
		t.Fatal(err)
	}
	if got := wait(); got.ID != id {
		t.Fatalf("retry changed id: %s → %s", id, got.ID)
	}
}

func TestEnqueueRetriesNetworkErrors(t *testing.T) {
	tr := transfers(session(t, "http://127.0.0.1:0"))
	tries := 0
	var changes []string
	tr.OnChange = func(status, _ string) { changes = append(changes, status) }
	tr.enqueue(domain.Transfer{Kind: "upload", Name: "x", Total: 3}, func(ctx context.Context, progress func(int64)) error {
		tries++
		if tries < 3 {
			return errors.New("connection reset")
		}
		progress(3)
		return nil
	})
	for i := 0; i < 200; i++ {
		if ts := tr.List(); ts[0].Status == "done" {
			if tries != 3 || ts[0].Done != 3 || strings.Join(changes, ",") != "queued,done" {
				t.Fatalf("tries=%d %+v changes=%v", tries, ts[0], changes)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("never finished: %+v", tr.List())
}

func TestTrashRestore(t *testing.T) {
	f := webdavtest.New(map[string]bool{"/": true, "/Reports": true}, map[string]string{"/Reports/a.pdf": "x", "/Reports/b.pdf": "y"})
	srv := httptest.NewServer(f)
	defer srv.Close()
	s := session(t, srv.URL)
	idx := NewIndex(s, noEvents{})
	trash, files := &Trash{S: s, Index: idx}, &Files{S: s, Index: idx}

	trashed, err := trash.Trash("/Reports/a.pdf")
	if err != nil || !strings.HasPrefix(trashed, "/.trash/") || f.Files["/Reports/a.pdf"] != "" {
		t.Fatalf("trash: %v %s", err, trashed)
	}
	if root, _ := files.List("/"); slices.ContainsFunc(root, func(e domain.Entry) bool { return e.Name == ".trash" }) {
		t.Fatal(".trash must stay hidden from listings")
	}
	items, err := trash.List()
	if err != nil || len(items) != 1 || items[0].Name != "a.pdf" || items[0].From != "/Reports" {
		t.Fatalf("list trash: %v %+v", err, items)
	}
	f.Files["/Reports/a.pdf"] = "z"
	if err := trash.Restore(trashed); err != nil || f.Files["/Reports/a 2.pdf"] != "x" || len(f.Dirs) != 3 {
		t.Fatalf("restore: %v files=%v dirs=%v", err, f.Files, f.Dirs)
	}
}

func TestThumbFile(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 900, 300))
	for i := range src.Pix {
		src.Pix[i] = 200
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, src)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(buf.Bytes()) }))
	defer srv.Close()
	th := &Thumbs{S: session(t, srv.URL), Dir: t.TempDir()}
	file, err := th.File(context.Background(), "/a.png", "1")
	if err != nil {
		t.Fatal(err)
	}
	fh, _ := os.Open(file)
	img, err := jpeg.Decode(fh)
	fh.Close()
	if err != nil || img.Bounds().Dx() != 360 || img.Bounds().Dy() != 120 {
		t.Fatalf("thumb: err=%v bounds=%v", err, img.Bounds())
	}
	if n, _ := os.ReadDir(th.Dir); len(n) != 1 {
		t.Fatalf("expected one cached file, got %d", len(n))
	}
}
