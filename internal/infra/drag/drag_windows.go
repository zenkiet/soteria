//go:build windows

package drag

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"
)

var (
	OnWrite func(remote, dest string) error
	OnEnded func()

	shell32 = windows.NewLazySystemDLL("shell32.dll")
	ole32   = windows.NewLazySystemDLL("ole32.dll")

	shParseDisplayName                = shell32.NewProc("SHParseDisplayName")
	shCreateShellItemArrayFromIDLists = shell32.NewProc("SHCreateShellItemArrayFromIDLists")
	shDoDragDrop                      = shell32.NewProc("SHDoDragDrop")
	oleInitialize                     = ole32.NewProc("OleInitialize")
	oleUninitialize                   = ole32.NewProc("OleUninitialize")
	bhidDataObject                    = windows.GUID{Data1: 0xb8c0bd9f, Data2: 0xed24, Data3: 0x455c, Data4: [8]byte{0x83, 0xe6, 0xd5, 0x39, 0x0c, 0x4f, 0xe8, 0xc4}}
	iidDataObject                     = windows.GUID{Data1: 0x0000010e, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
)

// Only the IUnknown methods and the first IShellItemArray method are used.
type shellItemArray struct {
	vtbl *struct {
		w32.IUnknownVtbl
		BindToHandler w32.ComProc
	}
}

func dragEnded() {
	if OnEnded != nil {
		OnEnded()
	}
}

func hresultError(operation string, hr uintptr) error {
	if int32(hr) < 0 {
		return fmt.Errorf("%s: HRESULT 0x%08X", operation, uint32(hr))
	}
	return nil
}

// StartPaths must run on the window's UI thread during a drag gesture. window is
// an HWND (nil is allowed). Paths must be absolute mounted paths in one folder.
// The Shell reads the mounted files directly; OnWrite is not used.
func StartPaths(window unsafe.Pointer, paths []string) error {
	defer dragEnded()
	if len(paths) == 0 {
		return errors.New("no mounted paths to drag")
	}
	if uint64(len(paths)) > uint64(^uint32(0)) {
		return errors.New("too many mounted paths to drag")
	}
	parent := filepath.Dir(paths[0])
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("drag path must be absolute: %q", path)
		}
		// BHID_DataObject only supports flat arrays created from ID lists.
		if !strings.EqualFold(filepath.Dir(path), parent) {
			return errors.New("drag paths must belong to the same folder")
		}
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := oleInitialize.Call(0)
	if err := hresultError("OleInitialize (STA required)", hr); err != nil {
		return err
	}
	// S_FALSE is successful too and must also be balanced.
	defer oleUninitialize.Call()

	pidls := make([]unsafe.Pointer, len(paths))
	defer func() {
		for _, pidl := range pidls {
			windows.CoTaskMemFree(pidl)
		}
	}()
	for i, path := range paths {
		name, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return fmt.Errorf("invalid drag path %q: %w", path, err)
		}
		hr, _, _ = shParseDisplayName.Call(uintptr(unsafe.Pointer(name)), 0, uintptr(unsafe.Pointer(&pidls[i])), 0, 0)
		if err := hresultError("SHParseDisplayName", hr); err != nil {
			return fmt.Errorf("parse drag path %q: %w", path, err)
		}
	}

	var items *shellItemArray
	hr, _, _ = shCreateShellItemArrayFromIDLists.Call(uintptr(len(pidls)), uintptr(unsafe.Pointer(&pidls[0])), uintptr(unsafe.Pointer(&items)))
	if err := hresultError("SHCreateShellItemArrayFromIDLists", hr); err != nil {
		return err
	}
	defer items.vtbl.Release.Call(uintptr(unsafe.Pointer(items)))

	var data *w32.IUnknown
	hr, _, _ = items.vtbl.BindToHandler.Call(uintptr(unsafe.Pointer(items)), 0, uintptr(unsafe.Pointer(&bhidDataObject)), uintptr(unsafe.Pointer(&iidDataObject)), uintptr(unsafe.Pointer(&data)))
	if err := hresultError("IShellItemArray.BindToHandler(BHID_DataObject)", hr); err != nil {
		return err
	}
	defer data.Vtbl.Release.Call(uintptr(unsafe.Pointer(data)))

	var effect uint32
	// Since Vista, a nil IDropSource asks the Shell to create the drag source.
	hr, _, _ = shDoDragDrop.Call(uintptr(window), uintptr(unsafe.Pointer(data)), 0, uintptr(w32.DROPEFFECT_COPY), uintptr(unsafe.Pointer(&effect)))
	// Both DRAGDROP_S_DROP and DRAGDROP_S_CANCEL are successful completion.
	return hresultError("SHDoDragDrop", hr)
}
