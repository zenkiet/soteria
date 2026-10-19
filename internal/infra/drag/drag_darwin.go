//go:build darwin

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

// Start must run on the main thread with an NSWindow pointer.
func Start(window unsafe.Pointer, entries []domain.Entry, _ func() domain.Drive) error {
	b, _ := json.Marshal(entries)
	cs := C.CString(string(b))
	defer C.free(unsafe.Pointer(cs))
	if C.dragOut(window, cs) != 0 {
		return errors.New("no drag gesture in progress")
	}
	return nil
}

//export goDragWrite
func goDragWrite(remote, dest *C.char, size C.longlong, dir C.int) C.int {
	e := domain.Entry{Path: C.GoString(remote), Size: int64(size), Dir: dir != 0}
	if OnWrite == nil || OnWrite(e, C.GoString(dest)) != nil {
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
