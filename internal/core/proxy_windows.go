//go:build windows

package core

import (
	"syscall"

	"golang.org/x/sys/windows/registry"
)

func SetWinProxy(server string, enable bool) error {
	k, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		registry.SET_VALUE,
	)
	if err != nil {
		return err
	}
	defer k.Close()
	if enable {
		if err := k.SetStringValue("ProxyServer", server); err != nil {
			return err
		}
		if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
			return err
		}
	} else {
		if err := k.SetDWordValue("ProxyEnable", 0); err != nil {
			return err
		}
	}
	return notifyWinInet()
}

func notifyWinInet() error {
	mod := syscall.NewLazyDLL("wininet.dll")
	proc := mod.NewProc("InternetSetOptionW")
	const (
		settingsChanged = 39
		refresh         = 37
	)
	for _, op := range []uintptr{settingsChanged, refresh} {
		r, _, e := proc.Call(0, op, 0, 0)
		if r == 0 && e != nil && e != syscall.Errno(0) {
			return e
		}
	}
	return nil
}

func SetAutoStart(enable bool, exe string) error {
	k, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.SET_VALUE,
	)
	if err == nil {
		if enable {
			_ = k.DeleteValue("Ply") // old 1.0 key — elevated task replaces it
		} else {
			_ = k.DeleteValue("Ply")
		}
		k.Close()
	}
	return SetLogonTask(enable, exe)
}
