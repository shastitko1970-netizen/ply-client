//go:build !windows

package core

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func SetWinProxy(server string, enable bool) error {
	if runtime.GOOS == "darwin" {
		return setMacProxy(server, enable)
	}
	return setLinuxProxy(server, enable)
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

func setMacProxy(server string, enable bool) error {
	out, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err != nil {
		return fmt.Errorf("networksetup недоступен — на macOS прокси ставится им, туннель от этого не зависит")
	}
	host, port, splitErr := net.SplitHostPort(server)
	if enable && splitErr != nil {
		return fmt.Errorf("кривой адрес прокси")
	}
	var touched bool
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "asterisk") || strings.HasPrefix(line, "*") {
			continue
		}
		touched = true
		if enable {
			_ = exec.Command("networksetup", "-setwebproxy", line, host, port).Run()
			_ = exec.Command("networksetup", "-setsecurewebproxy", line, host, port).Run()
			_ = exec.Command("networksetup", "-setsocksfirewallproxy", line, host, port).Run()
			_ = exec.Command("networksetup", "-setwebproxystate", line, "on").Run()
			_ = exec.Command("networksetup", "-setsecurewebproxystate", line, "on").Run()
			_ = exec.Command("networksetup", "-setsocksfirewallproxystate", line, "on").Run()
		} else {
			_ = exec.Command("networksetup", "-setwebproxystate", line, "off").Run()
			_ = exec.Command("networksetup", "-setsecurewebproxystate", line, "off").Run()
			_ = exec.Command("networksetup", "-setsocksfirewallproxystate", line, "off").Run()
		}
	}
	if enable && !touched {
		return fmt.Errorf("networksetup не показал сетевых служб")
	}
	return nil
}

func setLinuxProxy(server string, enable bool) error {
	if _, err := exec.LookPath("gsettings"); err != nil {
		if !enable {
			return nil
		}
		return fmt.Errorf("нет gsettings — системный прокси умею на GNOME. Туннель работает без него")
	}
	if !enable {
		_ = exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "none").Run()
		return nil
	}
	host, port, err := net.SplitHostPort(server)
	if err != nil {
		return fmt.Errorf("кривой адрес прокси")
	}
	sets := [][]string{
		{"set", "org.gnome.system.proxy", "mode", "manual"},
		{"set", "org.gnome.system.proxy.http", "host", host},
		{"set", "org.gnome.system.proxy.http", "port", port},
		{"set", "org.gnome.system.proxy.https", "host", host},
		{"set", "org.gnome.system.proxy.https", "port", port},
		{"set", "org.gnome.system.proxy.socks", "host", host},
		{"set", "org.gnome.system.proxy.socks", "port", port},
		{"set", "org.gnome.system.proxy", "ignore-hosts", "['localhost', '127.0.0.0/8', '::1']"},
	}
	for _, args := range sets {
		if err := exec.Command("gsettings", args...).Run(); err != nil {
			return fmt.Errorf("gsettings не записал прокси: %w", err)
		}
	}
	return nil
}
