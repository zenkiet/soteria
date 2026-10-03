package wails

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestOnScreen(t *testing.T) {
	screens := []*application.Screen{
		{Bounds: application.Rect{X: 0, Y: 0, Width: 1920, Height: 1080}},
		// a monitor to the left
		{Bounds: application.Rect{X: -1440, Y: 0, Width: 1440, Height: 900}},
	}
	cases := []struct {
		x, y, w int
		want    bool
	}{
		// on primary
		{100, 100, 1280, true},
		// on the left monitor
		{-1400, 50, 1280, true},
		// on an unplugged right monitor
		{2500, 100, 1280, false},
		// above every screen
		{100, -2000, 1280, false},
	}
	for _, c := range cases {
		if got := onScreen(screens, c.x, c.y, c.w); got != c.want {
			t.Errorf("onScreen(%d,%d,%d) = %v, want %v", c.x, c.y, c.w, got, c.want)
		}
	}
	if !onScreen(nil, 9999, 9999, 100) {
		t.Error("empty screens must fail open")
	}
}
