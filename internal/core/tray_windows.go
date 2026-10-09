//go:build windows

package core

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
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
	procLoadIcon        = user32.NewProc("LoadIconW")
	procCreateIcon      = user32.NewProc("CreateIcon")
	procSHGetFileInfo   = shell32.NewProc("SHGetFileInfoW")
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
	hostHWND       windows.HWND
	trayIcon       windows.Handle
	trayHooks      TrayHooks
	wantQuit       atomic.Bool
	trayReady      atomic.Bool
	hidOnce        atomic.Bool
	trayYielded    atomic.Bool
	yieldMu        sync.Mutex
	yieldRunning   bool
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
		return
	}
	_, _, _ = procPostMessage.Call(uintptr(h), wmClose, 0, 0)
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
	trayIcon = loadTrayIcon(exe)

	nid := notifyIconData{
		HWnd:             hwnd,
		UID:              1,
		UFlags:           nifMessage | nifIcon | nifTip,
		UCallbackMessage: wmTray,
		HIcon:            trayIcon,
	}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	copyUTF16(nid.SzTip[:], "Ply VPN")
	ok, _, _ := procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
	if ok == 0 {
		if origWndProc != 0 {
			_, _, _ = procSetWindowLong.Call(uintptr(hwnd), gwlWndProc, origWndProc)
			origWndProc = 0
		}
		trayHWND = 0
		trayIcon = 0
		return false
	}
	trayReady.Store(true)
	return true
}

func RemoveTray() {
	if trayHWND != 0 {
		nid := notifyIconData{HWnd: trayHWND, UID: 1}
		nid.CbSize = uint32(unsafe.Sizeof(nid))
		_, _, _ = procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
		if origWndProc != 0 {
			_, _, _ = procSetWindowLong.Call(uintptr(trayHWND), gwlWndProc, origWndProc)
			origWndProc = 0
		}
	}
	trayHWND = 0
	trayIcon = 0
	trayReady.Store(false)
}

func YieldTray() {
	trayYielded.Store(true)
	RemoveTray()
	go watchLauncherGone()
}

func watchLauncherGone() {
	yieldMu.Lock()
	if yieldRunning {
		yieldMu.Unlock()
		return
	}
	yieldRunning = true
	yieldMu.Unlock()
	defer func() {
		yieldMu.Lock()
		yieldRunning = false
		yieldMu.Unlock()
	}()
	miss := 0
	for trayYielded.Load() {
		time.Sleep(2 * time.Second)
		if !trayYielded.Load() {
			return
		}
		if FindPlyHWND() != 0 {
			miss = 0
			continue
		}
		miss++
		if miss < 2 {
			continue
		}
		trayYielded.Store(false)
		if hostHWND != 0 && !trayReady.Load() {
			AttachTrayTo(hostHWND, trayHooks)
		}
		return
	}
}

type shFileInfo struct {
	HIcon         windows.Handle
	IIcon         int32
	DwAttributes  uint32
	SzDisplayName [260]uint16
	SzTypeName    [80]uint16
}

func loadTrayIcon(exe string) windows.Handle {
	cands := []string{exe}
	if exe != "" {
		dir := filepath.Dir(exe)
		cands = append(cands, filepath.Join(dir, "Ply.exe"), filepath.Join(dir, "PlyCore.exe"))
	}
	for _, p := range cands {
		if h := extractIcon(p); h != 0 {
			return h
		}
	}
	if exe != "" {
		const (
			shgfiIcon      = 0x100
			shgfiSmallIcon = 0x1
		)
		p, err := windows.UTF16PtrFromString(exe)
		if err == nil {
			var fi shFileInfo
			r, _, _ := procSHGetFileInfo.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&fi)), unsafe.Sizeof(fi), shgfiIcon|shgfiSmallIcon)
			if r != 0 && uintptr(fi.HIcon) > 1 {
				return fi.HIcon
			}
		}
	}
	if h := synthIcon(); h != 0 {
		return h
	}
	ic, _, _ := procLoadIcon.Call(0, 32512)
	return windows.Handle(ic)
}

func extractIcon(path string) windows.Handle {
	if path == "" {
		return 0
	}
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		return 0
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0
	}
	ic, _, _ := procExtractIcon.Call(0, uintptr(unsafe.Pointer(p)), 0)
	if ic <= 1 {
		return 0
	}
	return windows.Handle(ic)
}

func synthIcon() windows.Handle {
	const n = 16
	and := make([]byte, n*2)
	xor := make([]byte, n*2)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			bytei := y*2 + x/8
			bit := byte(0x80 >> (x % 8))
			ink := x == 0 || y == 0 || x == n-1 || y == n-1 ||
				(x >= 4 && x <= 6 && y >= 3 && y <= 12) ||
				(y >= 3 && y <= 5 && x >= 4 && x <= 11) ||
				(y >= 7 && y <= 9 && x >= 4 && x <= 10) ||
				(x >= 10 && x <= 11 && y >= 3 && y <= 9)
			if !ink {
				xor[bytei] |= bit
			}
		}
	}
	h, _, _ := procCreateIcon.Call(0, n, n, 1, 1, uintptr(unsafe.Pointer(&and[0])), uintptr(unsafe.Pointer(&xor[0])))
	return windows.Handle(h)
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
	appendMenu(m, cmdOn, "Включить")
	appendMenu(m, cmdOff, "Выключить")
	appendMenu(m, cmdShow, "Открыть Ply")
	appendMenu(m, cmdQuit, "Выйти и погасить")
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
