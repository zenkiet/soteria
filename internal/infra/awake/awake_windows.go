package awake

import (
	"runtime"
	"sync"

	"golang.org/x/sys/windows"
)

var setState = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetThreadExecutionState")

const (
	esContinuous     = 0x80000000
	esSystemRequired = 0x00000001
)

var (
	mu   sync.Mutex
	stop chan struct{}
)

// Hold pins one OS thread that asks Windows to stay awake; execution state belongs to the thread that set it.
func Hold(on bool) {
	mu.Lock()
	defer mu.Unlock()
	if on == (stop != nil) {
		return
	}
	if !on {
		close(stop)
		stop = nil
		return
	}
	stop = make(chan struct{})
	go func(stop chan struct{}) {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		_, _, _ = setState.Call(esContinuous | esSystemRequired)
		<-stop
		_, _, _ = setState.Call(esContinuous)
	}(stop)
}
