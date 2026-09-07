package store

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

var logPath = filepath.Join(Dir(), "soteria.log")

func LogPath() string { return logPath }

// Errors go to stderr and to a log file next to servers.json; the file rolls over once past 5 MB.
func init() {
	_ = os.MkdirAll(filepath.Dir(logPath), 0o700)
	if st, err := os.Stat(logPath); err == nil && st.Size() > 5<<20 {
		_ = os.Rename(logPath, logPath+".1")
	}
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
		log.SetOutput(io.MultiWriter(os.Stderr, f))
	}
}
