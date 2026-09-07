package mount

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
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

func freeLetter() string {
	for c := 'Z'; c >= 'D'; c-- {
		if _, err := os.Stat(string(c) + `:\`); err != nil {
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

// Mount shows the server as a local drive named Soteria through WinFsp.
func (m *Mounter) Mount() (domain.Drive, error) {
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
	host := fuse.NewFileSystemHost(newDavFS(m))
	host.SetCapReaddirPlus(true)
	done := make(chan bool, 1)
	go func() {
		done <- host.Mount(letter+":", []string{"-o", "volname=Soteria", "-o", "uid=-1", "-o", "gid=-1", "--FileSystemName=Soteria"})
	}()
	root := letter + `:\`
	for i := 0; i < 100; i++ {
		select {
		case <-done:
			return domain.Drive{}, errors.New("couldn't mount the drive, WinFsp refused it")
		default:
		}
		if _, err := os.Stat(root); err == nil {
			m.mu.Lock()
			mnt.host, mnt.letter = host, letter
			m.mu.Unlock()
			return domain.Drive{Path: root, Mounted: true}, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	host.Unmount()
	return domain.Drive{}, errors.New("the drive didn't appear in 10 s")
}

func (m *Mounter) Unmount() error {
	m.mu.Lock()
	host, letter := mnt.host, mnt.letter
	m.mu.Unlock()
	if host == nil {
		return nil
	}
	if !host.Unmount() {
		return errors.New("couldn't disconnect Soteria, close files that are still open and try again")
	}
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(letter + `:\`); err != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	m.mu.Lock()
	mnt.host, mnt.letter = nil, ""
	m.mu.Unlock()
	return nil
}

// InstallDriver downloads the pinned WinFsp installer, checks it, and runs it silently with elevation.
func (m *Mounter) InstallDriver() error {
	if driverInstalled() {
		return nil
	}
	resp, err := http.Get(winfspURL)
	if err != nil {
		return fmt.Errorf("couldn't download WinFsp: %w", err)
	}
	defer resp.Body.Close()
	msi := filepath.Join(os.TempDir(), "winfsp.msi")
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
		os.Remove(msi)
		return errors.New("the WinFsp download didn't match its checksum")
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	exe, _ := windows.UTF16PtrFromString("msiexec.exe")
	args, _ := windows.UTF16PtrFromString(`/i "` + msi + `" /qn INSTALLLEVEL=1000`)
	if err := windows.ShellExecute(0, verb, exe, args, nil, windows.SW_HIDE); err != nil {
		return fmt.Errorf("installing WinFsp was cancelled: %w", err)
	}
	for i := 0; i < 600; i++ {
		if driverInstalled() {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return errors.New("WinFsp didn't finish installing")
}
