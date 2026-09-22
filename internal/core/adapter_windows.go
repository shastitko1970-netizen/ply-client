//go:build windows

package core

import (
	"os/exec"
	"syscall"
	"time"
)

func PlyAdapterUp() bool {
	return plyAdapter() != nil
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
