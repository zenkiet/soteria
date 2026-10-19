package drag

import (
	"errors"
	"fmt"
	"path/filepath"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"

	"soteria/internal/domain"
)

var (
	shell32            = windows.NewLazySystemDLL("shell32.dll")
	ilCreateFromPath   = shell32.NewProc("ILCreateFromPathW")
	ilFree             = shell32.NewProc("ILFree")
	shCreateDataObject = shell32.NewProc("SHCreateDataObject")
	shDoDragDrop       = shell32.NewProc("SHDoDragDrop")
	oleInitialize      = windows.NewLazySystemDLL("ole32.dll").NewProc("OleInitialize")
	iidDataObject      = windows.GUID{Data1: 0x0000010e, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
)

// Start must run on the UI thread with an HWND; Explorer copies the items from the mounted drive.
// ponytail: needs the drive mounted, virtual files (CFSTR_FILEDESCRIPTOR) if dragging without it matters.
func Start(window unsafe.Pointer, entries []domain.Entry, drive func() domain.Drive) error {
	d := drive()
	if !d.Mounted {
		return errors.New("turn on the drive in Settings to drag items out")
	}
	// GetAsyncKeyState reads physical buttons; left-handed setups drag with the right one.
	vk := w32.VK_LBUTTON
	if w32.GetSystemMetrics(w32.SM_SWAPBUTTON) != 0 {
		vk = w32.VK_RBUTTON
	}
	if w32.GetAsyncKeyState(vk)&0x8000 == 0 {
		return errors.New("no drag gesture in progress")
	}
	pidls := make([]uintptr, len(entries))
	defer func() {
		for _, p := range pidls {
			_, _, _ = ilFree.Call(p)
		}
	}()
	for i, e := range entries {
		p, err := windows.UTF16PtrFromString(filepath.Join(d.Path, filepath.FromSlash(e.Path)))
		if err != nil {
			return err
		}
		if pidls[i], _, _ = ilCreateFromPath.Call(uintptr(unsafe.Pointer(p))); pidls[i] == 0 {
			return fmt.Errorf("%s isn't on the drive yet", e.Name)
		}
	}
	_, _, _ = oleInitialize.Call(0)
	var obj *w32.IUnknown
	if hr, _, _ := shCreateDataObject.Call(0, uintptr(len(pidls)), uintptr(unsafe.Pointer(&pidls[0])), 0, uintptr(unsafe.Pointer(&iidDataObject)), uintptr(unsafe.Pointer(&obj))); hr != 0 {
		return fmt.Errorf("couldn't start the drag (0x%x)", uint32(hr))
	}
	defer w32.ComRelease(obj)
	var effect uint32
	_, _, _ = shDoDragDrop.Call(uintptr(window), uintptr(unsafe.Pointer(obj)), 0, 1, uintptr(unsafe.Pointer(&effect)))
	if OnEnded != nil {
		OnEnded()
	}
	return nil
}
