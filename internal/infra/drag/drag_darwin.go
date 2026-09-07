//go:build darwin

// Package drag starts a native drag of remote files; the OS asks for each file on drop and OnWrite downloads it there.
package drag

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=11.0
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework UniformTypeIdentifiers
#include <stdlib.h>
int dragOut(void *window, const char *json);
*/
import "C"

import (
	"encoding/json"
	"errors"
	"unsafe"

	"soteria/internal/domain"
)

var (
	OnWrite func(remote, dest string) error // downloads remote to dest, blocking until done
	OnEnded func()
)

// Start must run on the main thread with an NSWindow pointer.
func Start(window unsafe.Pointer, entries []domain.Entry) error {
	b, _ := json.Marshal(entries)
	cs := C.CString(string(b))
	defer C.free(unsafe.Pointer(cs))
	if C.dragOut(window, cs) != 0 {
		return errors.New("no drag gesture in progress")
	}
	return nil
}

//export goDragWrite
func goDragWrite(remote, dest *C.char) C.int {
	if OnWrite == nil || OnWrite(C.GoString(remote), C.GoString(dest)) != nil {
		return 1
	}
	return 0
}

//export goDragEnded
func goDragEnded() {
	if OnEnded != nil {
		OnEnded()
	}
}
