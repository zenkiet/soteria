// Package webdavtest is an in-memory WebDAV server: enough of MKCOL/PUT/GET/MOVE/DELETE/PROPFIND for tests.
package webdavtest

import (
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type Server struct {
	Dirs  map[string]bool
	Files map[string]string
}

func New(dirs map[string]bool, files map[string]string) *Server {
	return &Server{Dirs: dirs, Files: files}
}

func (f *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimSuffix(r.URL.Path, "/")
	if p == "" {
		p = "/"
	}
	switch r.Method {
	case "MKCOL":
		if f.Dirs[p] {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		f.Dirs[p] = true
		w.WriteHeader(http.StatusCreated)
	case http.MethodPut:
		b, _ := io.ReadAll(r.Body)
		f.Files[p] = string(b)
		w.WriteHeader(http.StatusCreated)
	case http.MethodGet:
		b, ok := f.Files[p]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if rg := r.Header.Get("Range"); rg != "" {
			var from, to int
			_, _ = fmt.Sscanf(rg, "bytes=%d-%d", &from, &to)
			w.WriteHeader(http.StatusPartialContent)
			_, _ = io.WriteString(w, b[from:min(to+1, len(b))])
			return
		}
		_, _ = io.WriteString(w, b)
	case "MOVE":
		dst, _ := url.Parse(r.Header.Get("Destination"))
		to := strings.TrimSuffix(dst.Path, "/")
		if _, ok := f.Files[p]; !ok && !f.Dirs[p] {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if _, ok := f.Files[to]; ok || f.Dirs[to] {
			if r.Header.Get("Overwrite") != "T" {
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
			delete(f.Files, to)
		}
		for k, v := range f.Files {
			if under(k, p) {
				f.Files[to+k[len(p):]] = v
				delete(f.Files, k)
			}
		}
		for k := range f.Dirs {
			if under(k, p) {
				f.Dirs[to+k[len(p):]] = true
				delete(f.Dirs, k)
			}
		}
		w.WriteHeader(http.StatusCreated)
	case http.MethodDelete:
		maps.DeleteFunc(f.Files, func(k, _ string) bool { return under(k, p) })
		maps.DeleteFunc(f.Dirs, func(k string, _ bool) bool { return under(k, p) })
		w.WriteHeader(http.StatusNoContent)
	case "PROPFIND":
		if !f.Dirs[p] {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var b strings.Builder
		b.WriteString(`<?xml version="1.0"?><D:multistatus xmlns="DAV:" xmlns:D="DAV:">`)
		item := func(q string, dir bool) {
			b.WriteString(`<D:response><D:href>` + q + `</D:href><D:propstat><D:prop><D:resourcetype>`)
			if dir {
				b.WriteString(`<D:collection/>`)
			}
			b.WriteString(`</D:resourcetype>`)
			if !dir {
				fmt.Fprintf(&b, `<D:getcontentlength>%d</D:getcontentlength>`, len(f.Files[q]))
			}
			b.WriteString(`</D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>`)
		}
		item(p, true)
		for k := range f.Dirs {
			if path.Dir(k) == p && k != p {
				item(k, true)
			}
		}
		for k := range f.Files {
			if path.Dir(k) == p {
				item(k, false)
			}
		}
		b.WriteString(`</D:multistatus>`)
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, b.String())
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func under(k, p string) bool { return k == p || strings.HasPrefix(k, p+"/") }
