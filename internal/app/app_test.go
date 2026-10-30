package app

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
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
	idx.entries = map[string]item{}
	idx.Put(domain.Entry{Name: "Photos", Path: "/Photos", Dir: true}, domain.Entry{Name: "cat.png", Path: "/Photos/cat.png", Size: 5}, domain.Entry{Name: "notes.txt", Path: "/notes.txt"})
	if got := idx.Search("CAT", "", 0, false).Entries; len(got) != 1 || got[0].Path != "/Photos/cat.png" {
		t.Fatalf("search: %+v", got)
	}
	if got := idx.Search(" ", "", 0, false).Entries; len(got) != 0 {
		t.Fatalf("blank query should match nothing, got %d", len(got))
	}
	if got := idx.Recent(5); len(got) != 2 {
		t.Fatalf("recent skips folders: %+v", got)
	}
	if u := idx.Usages("/"); len(u) != 1 || u["/Photos"] != (domain.Usage{Known: true, Items: 1, Bytes: 5}) {
		t.Fatalf("usages: %+v", u)
	}
	if got := idx.Search("", "images", 0, false).Entries; len(got) != 1 || got[0].Path != "/Photos/cat.png" {
		t.Fatalf("type filter alone is a query: %+v", got)
	}
	if got := idx.Search("cat", "", time.Now().Unix(), false).Entries; len(got) != 0 {
		t.Fatalf("an unknown modified time is older than any since: %+v", got)
	}
	if n := idx.Bytes(); n != 5 {
		t.Fatalf("bytes: %d", n)
	}
	idx.Put(domain.Entry{Name: "cat", Path: "/cat", Dir: true}, domain.Entry{Name: "._cat.png", Path: "/._cat.png"})
	if f := idx.Search("cat", "", 0, false); f.Total != 2 || f.Entries[0].Path != "/cat" {
		t.Fatalf("exact name first, junk left out: %+v", f)
	}
	idx.Put(domain.Entry{Name: "config", Path: "/.git/config"})
	if got := idx.Recent(5); len(got) != 2 {
		t.Fatalf("recent skips junk and dot folders: %+v", got)
	}
	idx.Remove("/.git/config")
	idx.Remove("/cat")
	idx.Remove("/._cat.png")
	if at := idx.Status().At; at != 0 {
		t.Fatalf("never crawled, yet indexed at %d", at)
	}
	idx.Put(domain.Entry{Name: "Tài liệu Đà Nẵng.docx", Path: "/Tài liệu Đà Nẵng.docx"})
	if got := idx.Search("tai lieu da nang", "", 0, false).Entries; len(got) != 1 {
		t.Fatalf("accent-insensitive search: %+v", got)
	}
	idx.Remove("/Tài liệu Đà Nẵng.docx")
	idx.Move("/Photos", "/Pics", false)
	idx.Remove("/notes.txt")
	if got := idx.Search("cat", "", 0, false).Entries; len(got) != 1 || got[0].Path != "/Pics/cat.png" || len(idx.entries) != 2 {
		t.Fatalf("after move/remove: %+v", idx.entries)
	}
}

func TestQuotaFallsBackToIndex(t *testing.T) {
	srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>/</D:href><D:propstat><D:prop><D:quota-used-bytes>0</D:quota-used-bytes><D:quota-available-bytes>100</D:quota-available-bytes></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response></D:multistatus>`))
	}))
	srv.Start()
	s := session(t, srv.URL)
	idx := NewIndex(s, noEvents{})
	idx.entries = map[string]item{}
	idx.Put(domain.Entry{Name: "a", Path: "/a", Size: 7}, domain.Entry{Name: "d", Path: "/d", Dir: true})
	if q, err := (&Files{S: s, Index: idx}).Quota(); err != nil || q.Used != 7 || q.Available != 100 {
		t.Fatalf("quota: %+v %v", q, err)
	}
}

func TestUploadConflictsAndFreeName(t *testing.T) {
	srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>/a.png</D:href><D:propstat><D:prop><D:resourcetype/><D:getcontentlength>7</D:getcontentlength></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response></D:multistatus>`))
	}))
	srv.Start()

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
	id := tr.enqueue(domain.Transfer{Kind: "upload", Name: "x", Local: missing, Remote: "/x"}, upload(c, missing, "/x", 0)).ID
	if err := <-tr.ts[0].done; err == nil {
		t.Fatal("uploading a missing file must fail")
	}
	if err := tr.Retry(id); err != nil {
		t.Fatal(err)
	}
	if err := <-tr.ts[0].done; err == nil {
		t.Fatal("the retry must fail the same way")
	}
	if ts := tr.List(); len(ts) != 1 || ts[0].ID != id || ts[0].Status != "error" {
		t.Fatalf("retry: %+v", ts)
	}
}

func TestCancelWhileQueued(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr := transfers(&Session{})
		block := func(ctx context.Context, _ func(int64)) error { <-ctx.Done(); return ctx.Err() }
		for range 3 {
			defer tr.enqueue(domain.Transfer{}, block).cancel()
		}
		q := tr.enqueue(domain.Transfer{}, block)
		synctest.Wait()
		q.cancel()
		if err := <-q.done; !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled while queued: %v", err)
		}
		if n := tr.Running(); n != 3 {
			t.Fatalf("a cancelled queued transfer still counts: %d running", n)
		}
	})
}

type sent chan any

func (s sent) Emit(_ string, data any) { s <- data }

func TestEmitBatches(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ev := make(sent, 10)
		tr := NewTransfers(&Session{}, NewIndex(&Session{}, noEvents{}), ev)
		names := []string{"a", "b", "c", "d", "e"}
		for _, n := range names {
			tr.enqueue(domain.Transfer{Name: n}, func(context.Context, func(int64)) error { return nil })
		}
		time.Sleep(time.Second)
		if len(ev) != 1 {
			t.Fatalf("queued, running and done of 5 transfers should be one event, got %d", len(ev))
		}
		batch, _ := (<-ev).([]domain.Transfer)
		var got []string
		for _, x := range batch {
			if x.Status != "done" {
				t.Fatalf("batch holds a stale state: %+v", x)
			}
			got = append(got, x.Name)
		}
		if !slices.Equal(got, names) {
			t.Fatalf("batch order: %v", got)
		}
	})
}

func TestStallKeepsPart(t *testing.T) {
	srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10")
		_, _ = io.WriteString(w, "0123")
		_ = http.NewResponseController(w).Flush()
		<-r.Context().Done()
	}))
	srv.Start()
	c, _ := session(t, srv.URL).Client()
	local := filepath.Join(t.TempDir(), "f")
	ctx, stall := context.WithCancelCause(context.Background())
	_ = fetch(c, "/f", local)(ctx, func(n int64) {
		if n == 4 {
			stall(errStalled)
		}
	})
	if st, err := os.Stat(local + ".part"); err != nil || st.Size() != 4 {
		t.Fatalf("a stalled download must keep its .part for the retry: %v", err)
	}
}

func TestSentBodyIsNotAStall(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr := transfers(&Session{})
		err := <-tr.enqueue(domain.Transfer{Total: 3}, func(ctx context.Context, progress func(int64)) error {
			progress(3)
			select {
			case <-time.After(2 * stallAfter):
				return nil
			case <-ctx.Done():
				return context.Cause(ctx)
			}
		}).done
		if err != nil {
			t.Fatalf("waiting on the server after the last byte counted as a stall: %v", err)
		}
	})
}

func TestEnqueueRetriesNetworkErrors(t *testing.T) { synctest.Test(t, testEnqueueRetries) }

// The synctest bubble makes the 2 s and 4 s backoffs instant.
func testEnqueueRetries(t *testing.T) {
	tr := transfers(session(t, "http://127.0.0.1:0"))
	tries := 0
	var changes []string
	tr.OnChange = func(status, _ string) { changes = append(changes, status) }
	err := <-tr.enqueue(domain.Transfer{Kind: "upload", Name: "x", Total: 3}, func(ctx context.Context, progress func(int64)) error {
		tries++
		if tries < 3 {
			return errors.New("connection reset")
		}
		progress(3)
		return nil
	}).done
	if ts := tr.List(); err != nil || tries != 3 || ts[0].Status != "done" || ts[0].Done != 3 || strings.Join(changes, ",") != "queued,done" {
		t.Fatalf("err=%v tries=%d %+v changes=%v", err, tries, ts[0], changes)
	}
}

func TestFetchResumes(t *testing.T) {
	const body = "0123456789"
	var ranges []string
	srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ranges = append(ranges, r.Header.Get("Range"))
		w.Header().Set("ETag", `"v1"`)
		if r.Header.Get("Range") == "" {
			w.Header().Set("Content-Length", "10")
			_, _ = io.WriteString(w, body[:4])
			_ = http.NewResponseController(w).Flush()
			panic(http.ErrAbortHandler)
		}
		http.ServeContent(w, r, "", time.Time{}, strings.NewReader(body))
	}))
	srv.Start()
	c, _ := session(t, srv.URL).Client()
	local := filepath.Join(t.TempDir(), "f")
	run := fetch(c, "/f", local)
	if err := run(context.Background(), func(int64) {}); err == nil {
		t.Fatal("the first attempt should fail mid-body")
	}
	if err := run(context.Background(), func(int64) {}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(local); string(b) != body || ranges[1] != "bytes=4-" {
		t.Fatalf("got %q after ranges %q", b, ranges)
	}
}

func TestDragWriteFolder(t *testing.T) {
	f := webdavtest.New(map[string]bool{"/": true, "/R": true, "/R/sub": true, "/R/empty": true}, map[string]string{"/R/a": "x", "/R/sub/b": "y", "/R/README": "1", "/R/readme": "2"})
	srv := httptest.NewTestServer(t, f)
	srv.Start()
	tr := transfers(session(t, srv.URL))
	local := filepath.Join(t.TempDir(), "R")
	if err := tr.DragWrite(domain.Entry{Path: "/R", Dir: true}, local); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(filepath.Join(local, "a"))
	b, _ := os.ReadFile(filepath.Join(local, "sub", "b"))
	if _, err := os.Stat(filepath.Join(local, "empty")); err != nil || string(a) != "x" || string(b) != "y" {
		t.Fatalf("tree: a=%q b=%q empty=%v", a, b, err)
	}
	if ds, _ := os.ReadDir(local); len(ds) != 5 {
		t.Fatalf("names differing only in case should both land: %v", ds)
	}
}

func TestDownloadMakesMissingFolder(t *testing.T) {
	srv := httptest.NewTestServer(t, webdavtest.New(map[string]bool{"/": true}, map[string]string{"/a": "x"}))
	srv.Start()
	dest := filepath.Join(t.TempDir(), "gone", "Downloads")
	tr := transfers(session(t, srv.URL))
	if _, err := tr.Download("/a", 1, dest); err != nil {
		t.Fatal(err)
	}
	if err := <-tr.ts[0].done; err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "a")); string(b) != "x" {
		t.Fatalf("got %q", b)
	}
}

func TestLocalName(t *testing.T) {
	for _, bad := range []string{"..", "../evil", "/etc/passwd", ""} {
		if _, err := localName(bad); err == nil {
			t.Fatalf("%q escapes the download folder", bad)
		}
	}
	if p, err := localName("a/b.txt"); err != nil || p != filepath.Join("a", "b.txt") {
		t.Fatalf("localName: %q %v", p, err)
	}
	windowsNames = true
	defer func() { windowsNames = runtime.GOOS == "windows" }()
	if p, err := localName(`a:b/c?. `); err != nil || p != filepath.Join("a_b", "c_. _") {
		t.Fatalf("windows name: %q %v", p, err)
	}
}

func TestTrashRestore(t *testing.T) {
	f := webdavtest.New(map[string]bool{"/": true, "/Reports": true}, map[string]string{"/Reports/a.pdf": "x", "/Reports/b.pdf": "y"})
	srv := httptest.NewTestServer(t, f)
	srv.Start()
	s := session(t, srv.URL)
	idx := NewIndex(s, noEvents{})
	trash, files := &Trash{S: s, Index: idx}, &Files{S: s, Index: idx}

	trashed, err := trash.Trash("/Reports/a.pdf")
	if err != nil || !strings.HasPrefix(trashed, "/.trash/") || f.Files["/Reports/a.pdf"] != "" {
		t.Fatalf("trash: %v %s", err, trashed)
	}
	if root, _ := files.List(context.Background(), "/"); slices.ContainsFunc(root, func(e domain.Entry) bool { return e.Name == ".trash" }) {
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
	srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(buf.Bytes()) }))
	srv.Start()
	th := &Thumbs{S: session(t, srv.URL), Dir: t.TempDir()}
	file, err := th.File(context.Background(), "/a.png", "1")
	if err != nil {
		t.Fatal(err)
	}
	fh, _ := os.Open(file)
	img, format, err := image.Decode(fh)
	fh.Close()
	if err != nil || format != "png" || img.Bounds().Dx() != 360 || img.Bounds().Dy() != 120 {
		t.Fatalf("translucent source keeps alpha as a PNG: err=%v format=%s", err, format)
	}
	if n, _ := os.ReadDir(th.Dir); len(n) != 1 {
		t.Fatalf("expected one cached file, got %d", len(n))
	}
}
