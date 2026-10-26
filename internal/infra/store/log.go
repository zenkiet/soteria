package store

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

var logPath = filepath.Join(Dir(), "soteria.log")

func LogPath() string { return logPath }

// Log makes the default logger write to stderr and to a log file next to servers.json; the file rolls over once past 5 MB.
func Log() *slog.Logger {
	_ = os.MkdirAll(filepath.Dir(logPath), 0o700)
	if st, err := os.Stat(logPath); err == nil && st.Size() > 5<<20 {
		_ = os.Rename(logPath, logPath+".1")
	}
	var w io.Writer = os.Stderr
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
		// File first: a Windows GUI exe has no stderr, and MultiWriter stops at the first writer that fails.
		w = io.MultiWriter(f, os.Stderr)
	}
	l := slog.New(slog.NewTextHandler(w, nil))
	slog.SetDefault(l)
	return l
}
