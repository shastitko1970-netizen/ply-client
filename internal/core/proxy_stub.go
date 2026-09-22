//go:build !windows

package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func SetWinProxy(server string, enable bool) error {
	_, _ = server, enable
	return nil
}

func SetAutoStart(enable bool, exe string) error {
	if runtime.GOOS == "darwin" {
		return macAutoStart(enable, exe)
	}
	return linuxAutoStart(enable, exe)
}

func linuxAutoStart(enable bool, exe string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".config", "autostart")
	path := filepath.Join(dir, "ply.desktop")
	if !enable {
		_ = os.Remove(path)
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	body := "[Desktop Entry]\nType=Application\nName=Ply\nExec=" + exe + "\nX-GNOME-Autostart-enabled=true\n"
	return os.WriteFile(path, []byte(body), 0644)
}

func macAutoStart(enable bool, exe string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, "Library", "LaunchAgents")
	path := filepath.Join(dir, "land.ply.vpn.plist")
	if !enable {
		_ = exec.Command("launchctl", "unload", path).Run()
		_ = os.Remove(path)
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	body := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>land.ply.vpn</string>
<key>ProgramArguments</key><array><string>` + exe + `</string></array>
<key>RunAtLoad</key><true/>
</dict></plist>
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		return err
	}
	_ = exec.Command("launchctl", "load", "-w", path).Run()
	return nil
}
