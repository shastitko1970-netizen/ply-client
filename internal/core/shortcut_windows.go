//go:build windows

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ярлык: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func DesktopDir() string {
	if d := os.Getenv("USERPROFILE"); d != "" {
		return filepath.Join(d, "Desktop")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Desktop")
}

func StartMenuDir() string {
	if d := os.Getenv("APPDATA"); d != "" {
		return filepath.Join(d, "Microsoft", "Windows", "Start Menu", "Programs")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "Roaming", "Microsoft", "Windows", "Start Menu", "Programs")
}

func CommonStartMenuDir() string {
	if d := os.Getenv("ProgramData"); d != "" {
		return filepath.Join(d, "Microsoft", "Windows", "Start Menu", "Programs")
	}
	return `C:\ProgramData\Microsoft\Windows\Start Menu\Programs`
}
