//go:build windows

package core

import (
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func PlyAdapterUp() bool {
	var size uint32 = 15000
	for i := 0; i < 3; i++ {
		buf := make([]byte, size)
		aa := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, windows.GAA_FLAG_INCLUDE_PREFIX, 0, aa, &size)
		if err == windows.ERROR_BUFFER_OVERFLOW {
			continue
		}
		if err != nil {
			return false
		}
		for ; aa != nil; aa = aa.Next {
			desc := windows.UTF16PtrToString(aa.Description)
			name := windows.UTF16PtrToString(aa.FriendlyName)
			blob := strings.ToLower(desc + " " + name)
			if strings.Contains(blob, "ply") && aa.OperStatus == windows.IfOperStatusUp {
				return true
			}
		}
		return false
	}
	return false
}

func WaitPlyAdapter(d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if PlyAdapterUp() {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return PlyAdapterUp()
}

func AllowFirewall(exe string) {
	for _, dir := range []string{"in", "out"} {
		name := "Ply " + dir
		del := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", "name="+name)
		tuneCmd(del)
		_ = del.Run()
		add := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
			"name="+name, "dir="+dir, "action=allow", "program="+exe, "enable=yes", "profile=any")
		tuneCmd(add)
		_ = add.Run()
	}
}

func PreferAdapterMetric() {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command",
		`Get-NetAdapter | Where-Object { $_.InterfaceDescription -match 'Ply' -or $_.Name -match 'Ply' } | ForEach-Object { Set-NetIPInterface -InterfaceIndex $_.ifIndex -InterfaceMetric 1 -ErrorAction SilentlyContinue }`)
	tuneCmd(cmd)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	_ = cmd.Run()
}
