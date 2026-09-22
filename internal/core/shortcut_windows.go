//go:build windows

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func CreateShortcut(link, target, workdir, desc string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		return err
	}
	icon := target + ",0"
	script := fmt.Sprintf(
		"$s=(New-Object -ComObject WScript.Shell).CreateShortcut(%s); $s.TargetPath=%s; $s.WorkingDirectory=%s; $s.WindowStyle=1; $s.Description=%s; $s.IconLocation=%s; $s.Save()",
		psQuote(link), psQuote(target), psQuote(workdir), psQuote(desc), psQuote(icon),
	)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	tuneCmd(cmd)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ярлык: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	fi, err := os.Stat(link)
	if err != nil || fi.Size() < 64 {
		return fmt.Errorf("ярлык не записался: %s", link)
	}
	return nil
}

func known(id *windows.KNOWNFOLDERID, fallback func() string) string {
	p, err := windows.KnownFolderPath(id, 0)
	if err != nil || p == "" {
		return fallback()
	}
	return p
}

func DesktopDir() string {
	return known(windows.FOLDERID_Desktop, func() string {
		if d := os.Getenv("USERPROFILE"); d != "" {
			return filepath.Join(d, "Desktop")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Desktop")
	})
}

func StartMenuDir() string {
	return known(windows.FOLDERID_Programs, func() string {
		if d := os.Getenv("APPDATA"); d != "" {
			return filepath.Join(d, "Microsoft", "Windows", "Start Menu", "Programs")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AppData", "Roaming", "Microsoft", "Windows", "Start Menu", "Programs")
	})
}

func CommonStartMenuDir() string {
	return known(windows.FOLDERID_CommonPrograms, func() string {
		if d := os.Getenv("ProgramData"); d != "" {
			return filepath.Join(d, "Microsoft", "Windows", "Start Menu", "Programs")
		}
		return `C:\ProgramData\Microsoft\Windows\Start Menu\Programs`
	})
}

func InstallShortcuts(ply, dest string) error {
	if err := CreateShortcut(filepath.Join(DesktopDir(), "Ply.lnk"), ply, dest, "Ply VPN"); err != nil {
		return err
	}
	if err := CreateShortcut(filepath.Join(StartMenuDir(), "Ply.lnk"), ply, dest, "Ply VPN"); err != nil {
		return err
	}
	_ = CreateShortcut(filepath.Join(CommonStartMenuDir(), "Ply.lnk"), ply, dest, "Ply VPN")
	return nil
}
