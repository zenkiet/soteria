//go:build windows

package wails

import (
	"strings"
	"testing"

	"soteria/internal/domain"
	"soteria/internal/infra/mount"
)

func TestDragOutEmpty(t *testing.T) {
	if err := (&App{}).DragOut(nil); err != nil {
		t.Fatal(err)
	}
}

func TestDragOutDisconnected(t *testing.T) {
	for _, a := range []*App{{}, {M: &mount.Mounter{}}} {
		err := a.DragOut([]domain.Entry{{Path: "/file.txt"}})
		if err == nil || !strings.Contains(err.Error(), "connect and mount") {
			t.Fatalf("expected actionable disconnected error, got %v", err)
		}
	}
}
