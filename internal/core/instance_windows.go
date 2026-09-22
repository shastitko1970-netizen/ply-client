//go:build windows

package core

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var instanceMutex windows.Handle

func SetAppID() {
	s, err := windows.UTF16PtrFromString(AppUserModelID)
	if err != nil {
		return
	}
	_, _, _ = windows.NewLazySystemDLL("shell32.dll").NewProc("SetCurrentProcessExplicitAppUserModelID").Call(uintptr(unsafe.Pointer(s)))
}

func AcquireInstance() bool {
	name, err := windows.UTF16PtrFromString("Global\\PlyVPN")
	if err != nil {
		return true
	}
	h, err := windows.CreateMutex(nil, false, name)
	if err == windows.ERROR_ALREADY_EXISTS {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return false
	}
	if err != nil && h == 0 {
		return true
	}
	instanceMutex = h
	return true
}

func ActivateExisting() {
	hwnd := FindPlyHWND()
	if hwnd == 0 {
		return
	}
	showWindow(hwnd, swRestore)
	showWindow(hwnd, swShow)
	setForeground(hwnd)
}
