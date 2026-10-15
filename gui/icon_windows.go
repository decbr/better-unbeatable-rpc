package main

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// gearIconID is Windows' Settings gear in shell32.dll, by resource ID.
const gearIconID = 16826

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	procLoadImage    = user32.NewProc("LoadImageW")
	procSendMessage  = user32.NewProc("SendMessageW")
	procSystemMetric = user32.NewProc("GetSystemMetrics")
)

const (
	imageIcon   = 1
	wmSetIcon   = 0x0080
	iconSmall   = 0
	iconBig     = 1
	smCxIcon    = 11
	smCxSmIcon  = 49
	loadAsImage = 0x20
)

// setWindowIcon gives the app's windows the Settings gear.
func setWindowIcon() {
	shell32, err := windows.LoadLibraryEx("shell32.dll", 0, loadAsImage|windows.LOAD_LIBRARY_SEARCH_SYSTEM32)
	if err != nil {
		return
	}
	load := func(metric uintptr) uintptr {
		size, _, _ := procSystemMetric.Call(metric)
		h, _, _ := procLoadImage.Call(uintptr(shell32), gearIconID, imageIcon, size, size, 0)
		return h
	}
	big, small := load(smCxIcon), load(smCxSmIcon)
	if big == 0 || small == 0 {
		return
	}
	pid := uint32(os.Getpid())
	cb := windows.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		var owner uint32
		windows.GetWindowThreadProcessId(hwnd, &owner)
		if owner == pid {
			procSendMessage.Call(uintptr(hwnd), wmSetIcon, iconBig, big)
			procSendMessage.Call(uintptr(hwnd), wmSetIcon, iconSmall, small)
		}
		return 1
	})
	windows.EnumWindows(cb, unsafe.Pointer(nil))
}
