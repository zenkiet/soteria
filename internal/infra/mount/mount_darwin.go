package mount

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"soteria/internal/domain"
)

// mountDir is ~/Soteria: /Volumes is root-only, and Finder lists browsable network volumes from anywhere.
func mountDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Soteria")
}

func isMounted(p string) bool {
	var st syscall.Statfs_t
	if syscall.Statfs(p, &st) != nil {
		return false
	}
	name := make([]byte, 0, len(st.Fstypename))
	for _, c := range st.Fstypename {
		if c == 0 {
			break
		}
		name = append(name, byte(c))
	}
	return string(name) == "webdav"
}

func (m *Mounter) Drive() domain.Drive {
	return domain.Drive{Path: mountDir(), Mounted: isMounted(mountDir())}
}

// Mount shows the server in Finder as a volume named Soteria.
func (m *Mounter) Mount() (domain.Drive, error) {
	dir := mountDir()
	if isMounted(dir) {
		return domain.Drive{Path: dir, Mounted: true}, nil
	}
	if err := m.serveLoopback(); err != nil {
		return domain.Drive{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return domain.Drive{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "mount_webdav", "-S", "-v", "Soteria", "http://"+ln.Addr().String()+"/", dir).CombinedOutput()
	if ctx.Err() != nil {
		return domain.Drive{}, errors.New("couldn't connect the drive: Finder didn't answer in 20 s")
	}
	if err != nil {
		return domain.Drive{}, fmt.Errorf("couldn't connect the drive: %s", strings.TrimSpace(string(out)+" "+err.Error()))
	}
	return domain.Drive{Path: dir, Mounted: true}, nil
}

func (m *Mounter) Unmount() error {
	dir := mountDir()
	if !isMounted(dir) {
		return nil
	}
	if out, err := exec.Command("diskutil", "unmount", dir).CombinedOutput(); err != nil {
		return fmt.Errorf("couldn't eject Soteria: %s", strings.TrimSpace(string(out)))
	}
	_ = os.Remove(dir)
	return nil
}

func (m *Mounter) InstallDriver() error { return nil }

// ln is the loopback proxy's listener, set once under Mounter.mu.
var ln net.Listener

const loopbackPort = "34873"

// serveLoopback adds the login on 127.0.0.1 so the OS mounts the server without holding credentials.
func (m *Mounter) serveLoopback() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ln != nil {
		return nil
	}
	l, err := net.Listen("tcp", "127.0.0.1:"+loopbackPort)
	if err != nil {
		l, err = net.Listen("tcp", "127.0.0.1:0")
	}
	if err != nil {
		return err
	}
	ln = l
	// Only the header timeout: read/write deadlines would cut off large transfers.
	srv := &http.Server{Handler: http.HandlerFunc(m.proxy), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(l) }()
	return nil
}

func (m *Mounter) proxy(w http.ResponseWriter, r *http.Request) {
	// mount_webdav sends neither header and always the loopback Host; browsers (and DNS-rebound names) can't match that.
	if r.Host != ln.Addr().String() || r.Header.Get("Origin") != "" || r.Header.Get("Sec-Fetch-Mode") != "" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	c, err := m.client()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	local, base := "http://"+r.Host, strings.TrimSuffix(c.Base.String(), "/")
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(c.Base)
			pr.Out.SetBasicAuth(c.User, c.Pass)
			if dst, ok := strings.CutPrefix(pr.In.Header.Get("Destination"), local); ok {
				pr.Out.Header.Set("Destination", base+dst)
			}
		},
		Transport: c.HTTP.Transport,
	}
	rp.ServeHTTP(w, r)
}
