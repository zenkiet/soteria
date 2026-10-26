package mount

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/winfsp/cgofuse/fuse"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"soteria/internal/domain"
)

const (
	winfspURL = "https://github.com/winfsp/winfsp/releases/download/v2.1/winfsp-2.1.25156.msi"
	winfspSHA = "073a70e00f77423e34bed98b86e600def93393ba5822204fac57a29324db9f7a"
)

var errNoDriver = errors.New("the WinFsp driver is not installed")

var mnt struct {
	host   *fuse.FileSystemHost
	fs     *davFS
	letter string
}

func driverInstalled() bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WinFsp`, registry.QUERY_VALUE|registry.WOW64_32KEY)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue("InstallDir")
	return err == nil
}

// GetLogicalDrives also counts empty card readers and offline network drives, which Stat reports as free.
func freeLetter() string {
	used, _ := windows.GetLogicalDrives()
	for c := 'Z'; c >= 'D'; c-- {
		if used&(1<<(c-'A')) == 0 {
			return string(c)
		}
	}
	return ""
}

func (m *Mounter) Drive() domain.Drive {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mnt.letter == "" {
		return domain.Drive{}
	}
	return domain.Drive{Path: mnt.letter + `:\`, Mounted: true}
}

// Saving counts files written to the drive that haven't reached the server yet.
func (m *Mounter) Saving() int {
	m.mu.Lock()
	fs := mnt.fs
	m.mu.Unlock()
	if fs == nil {
		return 0
	}
	return len(fs.pending(func(string) bool { return true }))
}

// Mount shows the server as a local drive named Soteria through WinFsp.
func (m *Mounter) Mount() (domain.Drive, error) {
	m.busy.Lock()
	defer m.busy.Unlock()
	if d := m.Drive(); d.Mounted {
		return d, nil
	}
	if !driverInstalled() {
		return domain.Drive{}, errNoDriver
	}
	letter := freeLetter()
	if letter == "" {
		return domain.Drive{}, errors.New("no free drive letter")
	}
	fs := newDavFS(m)
	host := fuse.NewFileSystemHost(fs)
	host.SetCapReaddirPlus(true)
	done := make(chan bool, 1)
	go func() {
		// A WinFsp install whose DLL won't load panics here instead of failing.
		defer func() {
			if recover() != nil {
				done <- false
			}
		}()
		done <- host.Mount(letter+":", []string{"-o", "volname=Soteria", "-o", "uid=-1", "-o", "gid=-1", "--FileSystemName=Soteria"})
	}()
	root := letter + `:\`
	for range 100 {
		select {
		case <-done:
			return domain.Drive{}, errors.New("couldn't mount the drive, WinFsp refused it")
		default:
		}
		if _, err := os.Stat(root); err == nil {
			m.mu.Lock()
			mnt.host, mnt.fs, mnt.letter = host, fs, letter
			m.mu.Unlock()
			// The drive can also stop on its own; Drive must stop reporting it then.
			go func() {
				<-done
				m.mu.Lock()
				if mnt.host == host {
					mnt.host, mnt.fs, mnt.letter = nil, nil, ""
				}
				m.mu.Unlock()
			}()
			return domain.Drive{Path: root, Mounted: true}, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	host.Unmount()
	return domain.Drive{}, errors.New("the drive didn't appear in 10 s")
}

func (m *Mounter) Unmount() error {
	m.busy.Lock()
	defer m.busy.Unlock()
	m.mu.Lock()
	host, letter := mnt.host, mnt.letter
	m.mu.Unlock()
	if host == nil {
		return nil
	}
	if m.Saving() > 0 {
		return errors.New("files written to the drive are still uploading, try again when they are done")
	}
	if !host.Unmount() {
		return errors.New("couldn't disconnect Soteria, close files that are still open and try again")
	}
	for range 100 {
		if _, err := os.Stat(letter + `:\`); err != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	m.mu.Lock()
	mnt.host, mnt.fs, mnt.letter = nil, nil, ""
	m.mu.Unlock()
	return nil
}

// InstallDriver downloads the pinned WinFsp installer, checks it, and runs it with a progress bar; Windows asks for permission.
func (m *Mounter) InstallDriver() error {
	if driverInstalled() {
		return nil
	}
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Get(winfspURL)
	if err != nil {
		return fmt.Errorf("couldn't download WinFsp: %w", err)
	}
	defer resp.Body.Close()
	msi := filepath.Join(os.TempDir(), "winfsp.msi")
	defer os.Remove(msi)
	f, err := os.Create(msi)
	if err != nil {
		return err
	}
	sum := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, sum), resp.Body)
	f.Close()
	if err != nil {
		return err
	}
	if hex.EncodeToString(sum.Sum(nil)) != winfspSHA {
		return errors.New("the WinFsp download didn't match its checksum")
	}
	// A silent install can't show the elevation prompt; basic UI can, and msiexec's exit code says how it went.
	code := 0
	if err := exec.Command("msiexec", "/i", msi, "/qb", "INSTALLLEVEL=1000").Run(); err != nil {
		ee, ok := errors.AsType[*exec.ExitError](err)
		if !ok {
			return err
		}
		code = ee.ExitCode()
	}
	switch code {
	case 0, 3010:
		return nil
	case 1602:
		return errors.New("installing WinFsp was cancelled")
	}
	return fmt.Errorf("installing WinFsp failed (msiexec error %d)", code)
}
