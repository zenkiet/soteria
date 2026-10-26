//go:build !windows

package store

import (
	"os"
	"path/filepath"
)

func Downloads() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Downloads")
}
