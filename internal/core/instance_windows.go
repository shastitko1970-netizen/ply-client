//go:build windows

package core

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var instanceMutex windows.Handle
var coreMutex windows.Handle

func SetAppID() {
	s, err := windows.UTF16PtrFromString(AppUserModelID)
	if err != nil {
		return
	}
	_, _, _ = windows.NewLazySystemDLL("shell32.dll").NewProc("SetCurrentProcessExplicitAppUserModelID").Call(uintptr(unsafe.Pointer(s)))
}

func acquireNamed(name string, slot *windows.Handle) bool {
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return true
	}
	h, err := windows.CreateMutex(nil, false, n)
	if err == windows.ERROR_ALREADY_EXISTS {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return false
	}
	if err != nil && h == 0 {
		return true
	}
	*slot = h
	return true
}

func AcquireInstance() bool {
	return acquireNamed("Global\\PlyVPN.UI", &instanceMutex)
}

func AcquireCoreInstance() bool {
	return acquireNamed("Global\\PlyVPN.Core", &coreMutex)
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
