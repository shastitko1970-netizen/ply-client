//go:build windows

package core

import (
	"os"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmClose     = 0x0010
	wmCommand   = 0x0111
	wmLButtonUp = 0x0202
	wmRButtonUp = 0x0205
	wmTray      = 0x0400 + 32
	swHide      = 0
	swShow      = 5
	swRestore   = 9
	nimAdd      = 0
	nimModify   = 1
	nimDelete   = 2
	nifMessage  = 0x00000001
	nifIcon     = 0x00000002
	nifTip      = 0x00000004
	nifInfo     = 0x00000010
	niifUser    = 0x00000004
	niifNosound = 0x00000010
	mfString    = 0
	tpmRight    = 0x0008
	tpmBottom   = 0x0020
	tpmReturn   = 0x0100
	gwlWndProc  = ^uintptr(3) // -4, GWLP_WNDPROC
	cmdShow     = 1001
	cmdOff      = 1002
	cmdQuit     = 1003
	cmdOn       = 1004
)

type notifyIconData struct {
	CbSize           uint32
	HWnd             windows.HWND
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            windows.Handle
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GUID             windows.GUID
	HBalloonIcon     windows.Handle
}

type TrayHooks struct {
	OnShow       func()
	OnConnect    func()
	OnDisconnect func()
	OnQuit       func()
	Invalidate   func()
}

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	shell32             = windows.NewLazySystemDLL("shell32.dll")
	procFindWindowW     = user32.NewProc("FindWindowW")
	procShowWindow      = user32.NewProc("ShowWindow")
	procSetForeground   = user32.NewProc("SetForegroundWindow")
	procGetWindowLong   = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLong   = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProc  = user32.NewProc("CallWindowProcW")
	procShellNotifyIcon = shell32.NewProc("Shell_NotifyIconW")
	procExtractIcon     = shell32.NewProc("ExtractIconW")
	procCreatePopupMenu = user32.NewProc("CreatePopupMenu")
	procAppendMenu      = user32.NewProc("AppendMenuW")
	procTrackPopup      = user32.NewProc("TrackPopupMenu")
	procGetCursor       = user32.NewProc("GetCursorPos")
	procDestroyMenu     = user32.NewProc("DestroyMenu")
	procSetForeground2  = user32.NewProc("SetForegroundWindow")
	procPostMessage     = user32.NewProc("PostMessageW")

	traySubclassCB = windows.NewCallback(traySubclass)
	origWndProc    uintptr
	trayHWND       windows.HWND
	trayIcon       windows.Handle
	trayHooks      TrayHooks
	wantQuit       atomic.Bool
	trayReady      atomic.Bool
	hidOnce        atomic.Bool
)

type point struct{ X, Y int32 }

func FindPlyHWND() windows.HWND {
	title, _ := windows.UTF16PtrFromString(WindowTitle)
	r, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(title)))
	return windows.HWND(r)
}

func showWindow(h windows.HWND, cmd int) {
	_, _, _ = procShowWindow.Call(uintptr(h), uintptr(cmd))
}

func setForeground(h windows.HWND) {
	_, _, _ = procSetForeground.Call(uintptr(h))
}

func HideToTray() {
	h := FindPlyHWND()
	if h == 0 {
		h = trayHWND
	}
	if h != 0 {
		showWindow(h, swHide)
		if hidOnce.CompareAndSwap(false, true) {
			TrayBalloon("Ply", "Свёрнут в трей. Туннель не гаснет. Выход — из значка у часов.")
		}
	}
}

func ShowFromTray() {
	h := FindPlyHWND()
	if h != 0 {
		showWindow(h, swRestore)
		showWindow(h, swShow)
		setForeground(h)
	} else if trayHooks.OnShow != nil {
		trayHooks.OnShow()
	}
	if trayHooks.Invalidate != nil {
		trayHooks.Invalidate()
	}
}

func RequestQuit() {
	wantQuit.Store(true)
	h := trayHWND
	if h == 0 {
		h = FindPlyHWND()
	}
	if h != 0 {
		_, _, _ = procPostMessage.Call(uintptr(h), wmClose, 0, 0)
	}
}

func AttachTray(hooks TrayHooks) bool {
	return AttachTrayTo(FindPlyHWND(), hooks)
}

func AttachTrayTo(hwnd windows.HWND, hooks TrayHooks) bool {
	if trayReady.Load() {
		return true
	}
	if hwnd == 0 {
		return false
	}
	trayHooks = hooks
	trayHWND = hwnd
	orig, _, _ := procGetWindowLong.Call(uintptr(hwnd), gwlWndProc)
	origWndProc = orig
	_, _, _ = procSetWindowLong.Call(uintptr(hwnd), gwlWndProc, traySubclassCB)

	exe, _ := os.Executable()
	exe16, _ := windows.UTF16PtrFromString(exe)
	ic, _, _ := procExtractIcon.Call(0, uintptr(unsafe.Pointer(exe16)), 0)
	trayIcon = windows.Handle(ic)

	nid := notifyIconData{
		HWnd:             hwnd,
		UID:              1,
		UFlags:           nifMessage | nifIcon | nifTip,
		UCallbackMessage: wmTray,
		HIcon:            trayIcon,
	}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	copyUTF16(nid.SzTip[:], "Ply VPN")
	_, _, _ = procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
	trayReady.Store(true)
	return true
}

func RemoveTray() {
	if !trayReady.Load() {
		return
	}
	nid := notifyIconData{HWnd: trayHWND, UID: 1}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	_, _, _ = procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
	trayReady.Store(false)
}

func TrayBalloon(title, msg string) {
	if !trayReady.Load() {
		return
	}
	nid := notifyIconData{
		HWnd:        trayHWND,
		UID:         1,
		UFlags:      nifInfo | nifIcon | nifTip | nifMessage,
		HIcon:       trayIcon,
		DwInfoFlags: niifUser | niifNosound,
	}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.UCallbackMessage = wmTray
	copyUTF16(nid.SzTip[:], "Ply VPN")
	copyUTF16(nid.SzInfoTitle[:], title)
	copyUTF16(nid.SzInfo[:], msg)
	_, _, _ = procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

func copyUTF16(dst []uint16, s string) {
	v, _ := windows.UTF16FromString(s)
	n := len(v)
	if n > len(dst) {
		n = len(dst)
		v = v[:n]
		v[n-1] = 0
	}
	copy(dst, v)
}

func traySubclass(hwnd, msg, wparam, lparam uintptr) uintptr {
	switch msg {
	case wmClose:
		if !wantQuit.Load() {
			if h := FindPlyHWND(); h != 0 {
				showWindow(h, swHide)
				if hidOnce.CompareAndSwap(false, true) {
					TrayBalloon("Ply", "Окно закрыто. Туннель живой. Выход — из значка у часов.")
				}
			}
			return 0
		}
	case wmTray:
		switch lparam {
		case wmLButtonUp:
			ShowFromTray()
			return 0
		case wmRButtonUp:
			showTrayMenu(windows.HWND(hwnd))
			return 0
		}
	case wmCommand:
		switch wparam & 0xffff {
		case cmdShow:
			ShowFromTray()
		case cmdOn:
			if trayHooks.OnConnect != nil {
				trayHooks.OnConnect()
			}
		case cmdOff:
			if trayHooks.OnDisconnect != nil {
				trayHooks.OnDisconnect()
			}
			if trayHooks.Invalidate != nil {
				trayHooks.Invalidate()
			}
		case cmdQuit:
			if trayHooks.OnQuit != nil {
				trayHooks.OnQuit()
			}
			RequestQuit()
		}
		return 0
	}
	r, _, _ := procCallWindowProc.Call(origWndProc, hwnd, msg, wparam, lparam)
	return r
}

func showTrayMenu(hwnd windows.HWND) {
	m, _, _ := procCreatePopupMenu.Call()
	if m == 0 {
		return
	}
	defer procDestroyMenu.Call(m)
	appendMenu(m, cmdOn, "Включить VPN")
	appendMenu(m, cmdOff, "Выключить VPN")
	appendMenu(m, cmdShow, "Открыть Ply")
	appendMenu(m, cmdQuit, "Выйти и погасить туннель")
	var pt point
	_, _, _ = procGetCursor.Call(uintptr(unsafe.Pointer(&pt)))
	_, _, _ = procSetForeground2.Call(uintptr(hwnd))
	r, _, _ := procTrackPopup.Call(m, tpmRight|tpmBottom|tpmReturn, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(hwnd), 0)
	if r != 0 {
		_, _, _ = procPostMessage.Call(uintptr(hwnd), wmCommand, r, 0)
	}
}

func appendMenu(menu uintptr, id uintptr, text string) {
	p, _ := windows.UTF16PtrFromString(text)
	_, _, _ = procAppendMenu.Call(menu, mfString, id, uintptr(unsafe.Pointer(p)))
}
