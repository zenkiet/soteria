package wails

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"

	"soteria/internal/domain"
)

// windowsDragPaths uses Windows syntax on every host so validation is testable
// without a Windows GUI. The mounter supplies a drive-letter root, not a UNC path.
func windowsDragPaths(drive domain.Drive, entries []domain.Entry) ([]string, error) {
	if !drive.Mounted || drive.Path == "" {
		return nil, errors.New("drag out requires a connected mounted drive; connect and mount the drive first")
	}
	root := drive.Path
	if len(root) != 3 || ((root[0] < 'A' || root[0] > 'Z') && (root[0] < 'a' || root[0] > 'z')) || root[1:] != `:\` {
		return nil, errors.New("drag out requires a valid mounted drive root")
	}
	paths := make([]string, 0, len(entries))
	var parent string
	for i, entry := range entries {
		if entry.Dir {
			return nil, errors.New("drag out on Windows supports files only, not folders")
		}
		p := entry.Path
		if !strings.HasPrefix(p, "/") || !utf8.ValidString(p) {
			return nil, fmt.Errorf("invalid drag path %q: expected an absolute slash path", p)
		}
		for _, segment := range strings.Split(p[1:], "/") {
			if !windowsDragSegment(segment) {
				return nil, fmt.Errorf("invalid drag path %q: unsafe Windows path segment %q", p, segment)
			}
		}
		// BHID_DataObject only supports files from one parent. Compare remote
		// parents exactly: case folding could alias distinct server directories.
		if i == 0 {
			parent = path.Dir(p)
		} else if path.Dir(p) != parent {
			return nil, errors.New("drag out on Windows requires files from the same folder")
		}
		paths = append(paths, root+strings.ReplaceAll(p[1:], "/", `\`))
	}
	return paths, nil
}

func windowsDragSegment(segment string) bool {
	if segment == "" || strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " ") {
		return false
	}
	for _, c := range segment {
		if c < 32 || strings.ContainsRune(`\:<>"|?*`, c) {
			return false
		}
	}
	base, _, _ := strings.Cut(segment, ".")
	base = strings.ToUpper(strings.TrimRight(base, " "))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return false
	}
	if strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT") {
		suffix := strings.TrimPrefix(strings.TrimPrefix(base, "COM"), "LPT")
		if utf8.RuneCountInString(suffix) == 1 && strings.ContainsAny(suffix, "123456789¹²³") {
			return false
		}
	}
	return true
}
