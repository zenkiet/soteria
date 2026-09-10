//go:build windows

package wails

import (
	"testing"
)

func TestDragOutEmpty(t *testing.T) {
	if err := (&App{}).DragOut(nil); err != nil {
		t.Fatal(err)
	}
}
