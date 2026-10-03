//go:build windows

package wails

import (
	"encoding/json"
	"os"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"
)

var (
	user32si            = windows.NewLazySystemDLL("user32.dll")
	pFindWindowEx       = user32si.NewProc("FindWindowExW")
	pSendMessageTimeout = user32si.NewProc("SendMessageTimeoutW")
)

// Wails' FindWindow can't see message-only windows, so relaunches never woke the app; delete once fixed upstream.
func notifyFirstInstance() {
	if os.Getenv("WAILS_UPDATER_HELPER") == "1" {
		// The updater helper must reach application.New to swap the binary.
		return
	}
	id := "wails-app-" + singleInstanceID
	cls, _ := windows.UTF16PtrFromString(id + "-sic")
	name, _ := windows.UTF16PtrFromString(id + "-siw")
	// HWND_MESSAGE is (HWND)-3.
	const hwndMessage = ^uintptr(2)
	hwnd, _, _ := pFindWindowEx.Call(hwndMessage, 0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(name)))
	if hwnd == 0 {
		return
	}
	wd, _ := os.Getwd()
	data, err := json.Marshal(application.SecondInstanceData{Args: os.Args, WorkingDir: wd})
	if err != nil {
		return
	}
	payload, err := windows.UTF16FromString(string(data))
	if err != nil {
		return
	}
	cds := w32.COPYDATASTRUCT{
		DwData: w32.WMCOPYDATA_SINGLE_INSTANCE_DATA,
		CbData: uint32(len(payload) * 2),
		LpData: uintptr(unsafe.Pointer(&payload[0])),
	}
	const wmCopyData, smtoAbortIfHung = 0x004A, 0x0002
	var res uintptr
	// The timeout keeps a hung first instance from stranding this process.
	_, _, _ = pSendMessageTimeout.Call(hwnd, wmCopyData, 0, uintptr(unsafe.Pointer(&cds)), smtoAbortIfHung, 5000, uintptr(unsafe.Pointer(&res)))
	os.Exit(0)
}
