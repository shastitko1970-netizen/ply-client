//go:build windows

package core

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type wndClassEx struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     windows.Handle
	HIcon         windows.Handle
	HCursor       windows.Handle
	HbrBackground windows.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       windows.Handle
}

type winMsg struct {
	Hwnd    windows.HWND
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

var (
	kernel32            = windows.NewLazySystemDLL("kernel32.dll")
	procGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
	procRegisterClass   = user32.NewProc("RegisterClassExW")
	procCreateWindow    = user32.NewProc("CreateWindowExW")
	procGetMessage      = user32.NewProc("GetMessageW")
	procTranslate       = user32.NewProc("TranslateMessage")
	procDispatch        = user32.NewProc("DispatchMessageW")
	procDefWnd          = user32.NewProc("DefWindowProcW")
	hostWndProcCB       = windows.NewCallback(hostWndProc)
)

func hostWndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	r, _, _ := procDefWnd.Call(hwnd, msg, wparam, lparam)
	return r
}

func RunHost(hooks TrayHooks) {
	hInst, _, _ := procGetModuleHandle.Call(0)
	clsName, _ := windows.UTF16PtrFromString("PlyCoreHost")
	wc := wndClassEx{
		LpfnWndProc:   hostWndProcCB,
		HInstance:     windows.Handle(hInst),
		LpszClassName: clsName,
	}
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	_, _, _ = procRegisterClass.Call(uintptr(unsafe.Pointer(&wc)))

	title, _ := windows.UTF16PtrFromString(CoreWindowTitle)
	const (
		wsExTool     = 0x00000080
		wsExNoact    = 0x08000000
		wsPopup      = 0x80000000
		cwUseDefault = 0x80000000
	)
	hwndU, _, _ := procCreateWindow.Call(
		wsExTool|wsExNoact,
		uintptr(unsafe.Pointer(clsName)),
		uintptr(unsafe.Pointer(title)),
		wsPopup,
		cwUseDefault, cwUseDefault, 0, 0,
		0, 0, hInst, 0,
	)
	hwnd := windows.HWND(hwndU)
	if hwnd != 0 {
		AttachTrayTo(hwnd, hooks)
	}
	var m winMsg
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		_, _, _ = procTranslate.Call(uintptr(unsafe.Pointer(&m)))
		_, _, _ = procDispatch.Call(uintptr(unsafe.Pointer(&m)))
	}
	RemoveTray()
}
