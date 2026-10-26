package store

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// Downloads follows the folder Windows has on record; people often move it to another drive.
func Downloads() string {
	if p, err := windows.KnownFolderPath(windows.FOLDERID_Downloads, 0); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Downloads")
}
