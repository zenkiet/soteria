package webdav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"soteria/internal/domain"
)

const sample = `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">
 <D:response><D:href>/dav/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype><D:quota-used-bytes>1000</D:quota-used-bytes><D:quota-available-bytes>9000</D:quota-available-bytes></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>
 <D:response><D:href>/dav/Product%20Photos/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype><D:getlastmodified>Fri, 04 Sep 2026 10:32:00 GMT</D:getlastmodified><D:creationdate>2026-02-03T10:15:00Z</D:creationdate></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>
 <D:response><D:href>/dav/store-front.jpg</D:href>
  <D:propstat><D:prop><D:resourcetype/><D:getcontentlength>3548112</D:getcontentlength><D:getcontenttype>image/jpeg</D:getcontenttype><D:getetag>"abc"</D:getetag></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat>
  <D:propstat><D:prop><D:quota-used-bytes/></D:prop><D:status>HTTP/1.1 404 Not Found</D:status></D:propstat>
 </D:response>
</D:multistatus>`

func TestListAndQuota(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, _ := r.BasicAuth(); u != "pos" || p != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(sample))
	}))
	defer srv.Close()

	c, err := New(domain.Server{URL: srv.URL + "/dav/", Username: "pos"}, "secret")
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.List(context.Background(), "/")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].Dir || got[0].Path != "/Product Photos" || got[0].Modified.IsZero() || got[0].Created.IsZero() ||
		got[1].Dir || got[1].Path != "/store-front.jpg" || got[1].Size != 3548112 || got[1].ContentType != "image/jpeg" {
		t.Fatalf("unexpected entries: %+v", got)
	}
	q, err := c.Quota(context.Background())
	if err != nil || q.Used != 1000 || q.Available != 9000 {
		t.Fatalf("quota %+v, %v", q, err)
	}

	bad, _ := New(domain.Server{URL: srv.URL, Username: "pos"}, "wrong")
	if _, err := bad.List(context.Background(), "/"); err == nil || err.Error() != "invalid username or password" {
		t.Fatalf("want auth error, got %v", err)
	}
	if err := statusError(http.StatusPreconditionFailed, "MOVE", "/x"); err.Error() != "something with that name already exists there" {
		t.Fatalf("statusError: %v", err)
	}
}

// A server that refuses Depth: infinity forces the folder walk; the walk must still find everything.
func TestTreeFallback(t *testing.T) {
	tree := map[string][]string{"/": {"Photos/", "notes.txt"}, "/Photos/": {"cat.png"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Depth") == "infinity" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		p := r.URL.Path
		if p[len(p)-1] != '/' {
			p += "/"
		}
		w.WriteHeader(http.StatusMultiStatus)
		body := `<?xml version="1.0"?><D:multistatus xmlns:D="DAV:">`
		for _, child := range tree[p] {
			res := ""
			if child[len(child)-1] == '/' {
				res = "<D:collection/>"
			}
			body += `<D:response><D:href>` + p + child + `</D:href><D:propstat><D:prop><D:resourcetype>` + res + `</D:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>`
		}
		_, _ = w.Write([]byte(body + `</D:multistatus>`))
	}))
	defer srv.Close()

	c, _ := New(domain.Server{URL: srv.URL, Username: "u"}, "p")
	entries, err := c.Tree(context.Background(), "/", func(domain.IndexStatus) {})
	if err != nil || len(entries) != 3 {
		t.Fatalf("tree: %v %+v", err, entries)
	}
}

func TestRestQuota(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/quota/u" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"username":"u","quota_size":100,"used_quota_size":30}`))
		case r.Method == "PROPFIND":
			w.WriteHeader(http.StatusMultiStatus)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>/dav/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response></D:multistatus>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c, _ := New(domain.Server{URL: srv.URL + "/dav", Username: "u"}, "p")
	q, err := c.Quota(context.Background())
	if err != nil || q.Used != 30 || q.Available != 70 {
		t.Fatalf("quota via proxy: %+v %v", q, err)
	}
	c2, _ := New(domain.Server{URL: srv.URL + "/dav", Username: "other"}, "p")
	if q, _ := c2.Quota(context.Background()); q.Used != -1 {
		t.Fatalf("quota for unknown user should stay unknown, got %+v", q)
	}
}
