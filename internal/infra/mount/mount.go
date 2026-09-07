// Package mount shows the server as a drive: a loopback proxy that adds the login, then mount_webdav on macOS or WinFsp on Windows.
package mount

import (
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"

	"soteria/internal/infra/webdav"
)

type Mounter struct {
	client   func() (*webdav.Client, error)
	onChange func() // called after the drive writes to the server so the search index can follow
	mu       sync.Mutex
	ln       net.Listener
}

func New(client func() (*webdav.Client, error), onChange func()) *Mounter {
	return &Mounter{client: client, onChange: onChange}
}

const loopbackPort = "34873"

// serveLoopback starts a 127.0.0.1 WebDAV proxy that adds the current login, so the OS can mount the server without credentials.
func (m *Mounter) serveLoopback() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ln != nil {
		return nil
	}
	ln, err := net.Listen("tcp", "127.0.0.1:"+loopbackPort)
	if err != nil {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
	}
	if err != nil {
		return err
	}
	m.ln = ln
	// Only the header timeout: read/write deadlines would cut off large transfers.
	srv := &http.Server{Handler: http.HandlerFunc(m.proxy), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	return nil
}

func (m *Mounter) proxy(w http.ResponseWriter, r *http.Request) {
	c, err := m.client()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	local, base := "http://"+r.Host, strings.TrimSuffix(c.Base.String(), "/")
	rp := httputil.NewSingleHostReverseProxy(c.Base)
	direct := rp.Director
	rp.Director = func(r *http.Request) {
		direct(r)
		r.Host = c.Base.Host
		r.SetBasicAuth(c.User, c.Pass)
		if dst := r.Header.Get("Destination"); strings.HasPrefix(dst, local) {
			r.Header.Set("Destination", base+strings.TrimPrefix(dst, local))
		}
	}
	rp.Transport = c.HTTP.Transport
	rp.ServeHTTP(w, r)
}
