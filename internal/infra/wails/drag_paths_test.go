package wails

import (
	"reflect"
	"strings"
	"testing"

	"soteria/internal/domain"
)

func TestWindowsDragPaths(t *testing.T) {
	drive := domain.Drive{Mounted: true, Path: `Z:\`}
	for _, tt := range []struct {
		name    string
		entries []domain.Entry
		want    []string
	}{
		{"root file", []domain.Entry{{Path: "/file.txt"}}, []string{`Z:\file.txt`}},
		{"siblings", []domain.Entry{{Path: "/My Files/a.txt"}, {Path: "/My Files/猫.txt"}}, []string{`Z:\My Files\a.txt`, `Z:\My Files\猫.txt`}},
		{"literal percent", []domain.Entry{{Path: "/%2e%2e/%5c.txt"}}, []string{`Z:\%2e%2e\%5c.txt`}},
		{"ordinary names", []domain.Entry{{Path: "/.hidden/COM10.txt"}, {Path: "/.hidden/console.txt"}}, []string{`Z:\.hidden\COM10.txt`, `Z:\.hidden\console.txt`}},
		{"ignore display name", []domain.Entry{{Name: "../../escape", Path: "/safe.txt"}}, []string{`Z:\safe.txt`}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := windowsDragPaths(drive, tt.entries)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestWindowsDragPathsRejectsUnsafePaths(t *testing.T) {
	paths := []string{
		"", "/", "relative.txt", "//server/share", "/a//b", "/a/", "/./a", "/a/../b", "/../b",
		`C:\file`, `/C:/file`, `/a\b`, `/a:stream`, "/nul\x00.txt", "/control\x1f.txt", "/invalid\xff",
		"/a<b", "/a>b", `/a"b`, "/a|b", "/a?b", "/a*b", "/a.", "/a ", "/a./b", "/a /b",
		"/CON", "/con.txt", "/PRN", "/aux.log", "/NUL", "/COM1.txt", "/com9", "/LPT1", "/lpt9.txt",
		"/COM¹.txt", "/LPT²", "/com³", "/CONIN$", "/conout$.txt", "/CON .txt", "/NUL/child.txt",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			got, err := windowsDragPaths(domain.Drive{Mounted: true, Path: `Z:\`}, []domain.Entry{{Path: p}})
			if err == nil || got != nil {
				t.Fatalf("unsafe path accepted: %q => %q, %v", p, got, err)
			}
		})
	}
}

func TestWindowsDragPathsRejectsSelection(t *testing.T) {
	for _, tt := range []struct {
		name    string
		drive   domain.Drive
		entries []domain.Entry
		message string
	}{
		{"disconnected", domain.Drive{}, nil, "connect and mount"},
		{"unmounted", domain.Drive{Path: `Z:\`}, nil, "connect and mount"},
		{"missing root", domain.Drive{Mounted: true}, nil, "connect and mount"},
		{"relative root", domain.Drive{Mounted: true, Path: `Z:`}, nil, "valid mounted drive root"},
		{"UNC root", domain.Drive{Mounted: true, Path: `\\server\share\`}, nil, "valid mounted drive root"},
		{"device root", domain.Drive{Mounted: true, Path: `\\?\Z:\`}, nil, "valid mounted drive root"},
		{"folder", domain.Drive{Mounted: true, Path: `Z:\`}, []domain.Entry{{Path: "/folder", Dir: true}}, "files only"},
		{"different parents", domain.Drive{Mounted: true, Path: `Z:\`}, []domain.Entry{{Path: "/a/x"}, {Path: "/b/y"}}, "same folder"},
		{"parent case alias", domain.Drive{Mounted: true, Path: `Z:\`}, []domain.Entry{{Path: "/a/x"}, {Path: "/A/y"}}, "same folder"},
		{"invalid second file", domain.Drive{Mounted: true, Path: `Z:\`}, []domain.Entry{{Path: "/safe"}, {Path: "/../escape"}}, "unsafe Windows path segment"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := windowsDragPaths(tt.drive, tt.entries)
			if got != nil || err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("got %q, %v; want error containing %q", got, err, tt.message)
			}
		})
	}
}
