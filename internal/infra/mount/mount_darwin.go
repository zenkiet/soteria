package mount

import (
	"context"
	"errors"
	"fmt"
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
	out, err := exec.CommandContext(ctx, "mount_webdav", "-S", "-v", "Soteria", "http://"+m.ln.Addr().String()+"/", dir).CombinedOutput()
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
