//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	gwlStyle        = ^uintptr(15) // -16
	gwlWndProc      = ^uintptr(3)  // -4
	swpFrameChanged = 0x0020
	swpNoZOrder     = 0x0004
	swpNoMove       = 0x0002
	swpNoSize       = 0x0001
	swMinimize      = 6
	wmClose         = 0x0010
	wmSysCommand    = 0x0112
	wmNCHitTest     = 0x0084
	wmNCLbuttonDown = 0x00A1
	htClient        = 1
	htCaption       = 2
	scMove          = 0xF010
	wsPopup         = 0x80000000
	wsVisible       = 0x10000000
	wsThickFrame    = 0x00040000
	wsMinimizeBox   = 0x00020000
	wsSysMenu       = 0x00080000
	wsClipChildren  = 0x02000000
	wsClipSiblings  = 0x04000000
	dwmDark         = 20
	dwmCorners      = 33
	dwmBorder       = 34
	dwmCaption      = 35
	dwmText         = 36
	dwmRound        = 2
	colorBg         = 0x000B0A0A
	colorFg         = 0x00F5F4F4
)

var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	dwmapi            = windows.NewLazySystemDLL("dwmapi.dll")
	procGetWindowLong = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLong = user32.NewProc("SetWindowLongPtrW")
	procSetWindowPos  = user32.NewProc("SetWindowPos")
	procGetWindowRect = user32.NewProc("GetWindowRect")
	procCallWndProc   = user32.NewProc("CallWindowProcW")
	procShowWindow    = user32.NewProc("ShowWindow")
	procPostMessage   = user32.NewProc("PostMessageW")
	procSendMessage   = user32.NewProc("SendMessageW")
	procReleaseCap    = user32.NewProc("ReleaseCapture")
	procGetDpi        = user32.NewProc("GetDpiForWindow")
	procDwmSetAttr    = dwmapi.NewProc("DwmSetWindowAttribute")
	origWndProc       uintptr
	hitCallback       = windows.NewCallback(hitProc)
)

func dwmSet(hwnd uintptr, attr int, value uint32) {
	v := value
	_, _, _ = procDwmSetAttr.Call(hwnd, uintptr(attr), uintptr(unsafe.Pointer(&v)), 4)
}

func dressWindow(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	dwmSet(hwnd, dwmDark, 1)
	dwmSet(hwnd, dwmCorners, dwmRound)
	dwmSet(hwnd, dwmBorder, colorBg)
	dwmSet(hwnd, dwmCaption, colorBg)
	dwmSet(hwnd, dwmText, colorFg)

	style := uintptr(wsPopup | wsVisible | wsThickFrame | wsMinimizeBox | wsSysMenu | wsClipChildren | wsClipSiblings)
	_, _, _ = procSetWindowLong.Call(hwnd, gwlStyle, style)
	_, _, _ = procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoZOrder|swpFrameChanged)

	prev, _, _ := procGetWindowLong.Call(hwnd, gwlWndProc)
	origWndProc = prev
	_, _, _ = procSetWindowLong.Call(hwnd, gwlWndProc, hitCallback)
}

func hitProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	if msg == wmNCHitTest {
		r, _, _ := procCallWndProc.Call(origWndProc, hwnd, msg, wparam, lparam)
		if r == htClient {
			var rc struct{ L, T, R, B int32 }
			_, _, _ = procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
			x := int32(int16(lparam))
			y := int32(int16(lparam >> 16))
			dpi := uint32(96)
			if p, _, _ := procGetDpi.Call(hwnd); p != 0 {
				dpi = uint32(p)
			}
			cap := int32(36 * dpi / 96)
			btns := int32(88 * dpi / 96)
			if y >= rc.T && y < rc.T+cap && x < rc.R-btns {
				return htCaption
			}
		}
		return r
	}
	r, _, _ := procCallWndProc.Call(origWndProc, hwnd, msg, wparam, lparam)
	return r
}

func winMin(hwnd uintptr) {
	_, _, _ = procShowWindow.Call(hwnd, swMinimize)
}

func winClose(hwnd uintptr) {
	_, _, _ = procPostMessage.Call(hwnd, wmClose, 0, 0)
}

func winDrag(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	_, _, _ = procReleaseCap.Call()
	_, _, _ = procSendMessage.Call(hwnd, wmNCLbuttonDown, htCaption, 0)
}

func systemDPI() uint32 {
	p, _, _ := user32.NewProc("GetDpiForSystem").Call()
	if p == 0 {
		return 96
	}
	return uint32(p)
}

func dip(n int) int {
	d := systemDPI()
	if d < 96 {
		d = 96
	}
	return int(uint32(n) * d / 96)
}

func setOuterSize(hwnd uintptr, w, h int) {
	if hwnd == 0 || w <= 0 || h <= 0 {
		return
	}
	_, _, _ = procSetWindowPos.Call(hwnd, 0, 0, 0, uintptr(w), uintptr(h), swpNoMove|swpNoZOrder|swpFrameChanged)
}

func coInitSTA() {
	ole32 := windows.NewLazySystemDLL("ole32.dll")
	_, _, _ = ole32.NewProc("CoInitializeEx").Call(0, 2)
}
