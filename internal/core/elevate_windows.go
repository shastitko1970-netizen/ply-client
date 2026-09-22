//go:build windows

package core

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

func IsAdmin() bool {
	f, err := os.Open(`\\.\PHYSICALDRIVE0`)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

func RelaunchElevated() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	verb, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := syscall.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	cwd, err := syscall.UTF16PtrFromString("")
	if err != nil {
		return err
	}
	param, err := syscall.UTF16PtrFromString("")
	if err != nil {
		return err
	}
	r, _, e := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW").Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(param)),
		uintptr(unsafe.Pointer(cwd)),
		1, // SW_SHOWNORMAL
	)
	if r <= 32 {
		if e != nil && e != syscall.Errno(0) {
			return fmt.Errorf("запрос прав: %v", e)
		}
		return fmt.Errorf("запрос прав отклонён")
	}
	return nil
}
