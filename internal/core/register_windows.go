//go:build windows

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

func InstalledVersion() string {
	for _, dir := range []string{DefaultInstallDir(), LegacyInstallDir()} {
		if b, err := os.ReadFile(filepath.Join(dir, "version.txt")); err == nil {
			v := strings.TrimSpace(string(b))
			if v != "" {
				return v
			}
		}
		if _, err := os.Stat(filepath.Join(dir, "Ply.exe")); err == nil {
			return "1.0.0"
		}
	}
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		k, err := registry.OpenKey(root, `Software\Microsoft\Windows\CurrentVersion\Uninstall\Ply`, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		v, _, err := k.GetStringValue("DisplayVersion")
		k.Close()
		if err == nil && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func WriteVersionFile(dir string) error {
	return os.WriteFile(filepath.Join(dir, "version.txt"), []byte(Version+"\n"), 0644)
}

func StopPlyProcesses() {
	for _, name := range []string{"Ply.exe", "xray.exe"} {
		cmd := exec.Command("taskkill", "/F", "/IM", name, "/T")
		tuneCmd(cmd)
		_ = cmd.Run()
	}
}

func RegisterApp(dir string) error {
	exe := filepath.Join(dir, "Ply.exe")
	uninst := filepath.Join(dir, "Uninstall.bat")
	roots := []registry.Key{registry.CURRENT_USER}
	if IsAdmin() {
		roots = append(roots, registry.LOCAL_MACHINE)
	}
	var last error
	for _, root := range roots {
		k, _, err := registry.CreateKey(root, `Software\Microsoft\Windows\CurrentVersion\Uninstall\Ply`, registry.ALL_ACCESS)
		if err != nil {
			last = err
			continue
		}
		_ = k.SetStringValue("DisplayName", "Ply")
		_ = k.SetStringValue("DisplayVersion", Version)
		_ = k.SetStringValue("Publisher", "Ply")
		_ = k.SetStringValue("InstallLocation", dir)
		_ = k.SetStringValue("DisplayIcon", exe)
		_ = k.SetStringValue("UninstallString", uninst)
		_ = k.SetStringValue("URLInfoAbout", "https://github.com/shastitko1970-netizen/ply-client")
		_ = k.SetDWordValue("NoModify", 1)
		_ = k.SetDWordValue("NoRepair", 1)
		_ = k.SetDWordValue("EstimatedSize", 80000)
		k.Close()
		last = nil
	}
	_ = registerAppPath(exe, dir)
	return last
}

func registerAppPath(exe, dir string) error {
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\Ply.exe`, registry.ALL_ACCESS)
	if err != nil {
		k, _, err = registry.CreateKey(registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\Ply.exe`, registry.ALL_ACCESS)
		if err != nil {
			return err
		}
	}
	defer k.Close()
	if err := k.SetStringValue("", exe); err != nil {
		return err
	}
	return k.SetStringValue("Path", dir)
}

func WriteUninstall(dir string) error {
	body := "@echo off\r\n" +
		"schtasks /Delete /TN Ply /F >nul 2>&1\r\n" +
		"taskkill /F /IM Ply.exe /T >nul 2>&1\r\n" +
		"taskkill /F /IM xray.exe /T >nul 2>&1\r\n" +
		"reg delete \"HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Ply\" /f >nul 2>&1\r\n" +
		"reg delete \"HKLM\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Ply\" /f >nul 2>&1\r\n" +
		"reg delete \"HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run\" /v Ply /f >nul 2>&1\r\n" +
		"reg delete \"HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\App Paths\\Ply.exe\" /f >nul 2>&1\r\n" +
		"reg delete \"HKCU\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\App Paths\\Ply.exe\" /f >nul 2>&1\r\n" +
		"del /f /q \"%USERPROFILE%\\Desktop\\Ply.lnk\" >nul 2>&1\r\n" +
		"del /f /q \"%APPDATA%\\Microsoft\\Windows\\Start Menu\\Programs\\Ply.lnk\" >nul 2>&1\r\n" +
		"del /f /q \"%ProgramData%\\Microsoft\\Windows\\Start Menu\\Programs\\Ply.lnk\" >nul 2>&1\r\n" +
		"timeout /t 1 /nobreak >nul\r\n" +
		"cd /d %TEMP%\r\n" +
		"rmdir /s /q \"" + dir + "\"\r\n"
	return os.WriteFile(filepath.Join(dir, "Uninstall.bat"), []byte(body), 0755)
}

func NotifyShell() {
	mod := syscall.NewLazyDLL("shell32.dll")
	proc := mod.NewProc("SHChangeNotify")
	const (
		assocChanged = 0x08000000
		flush        = 0x1000
		idlist       = 0x0000
	)
	_, _, _ = proc.Call(assocChanged, idlist|flush, 0, 0)
}

func RemoveLegacyStartFolder() {
	for _, dir := range []string{
		filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "Ply"),
		filepath.Join(LegacyInstallDir()),
	} {
		_ = os.Remove(filepath.Join(dir, "Ply.lnk"))
		_ = os.Remove(dir)
	}
	// keep url.txt backup; remove stale binaries so Start doesn't pick LocalAppData
	old := LegacyInstallDir()
	if old != DefaultInstallDir() {
		_ = os.Remove(filepath.Join(old, "Ply.exe"))
		_ = os.Remove(filepath.Join(old, "xray.exe"))
	}
}

func SetLogonTask(enable bool, exe string) error {
	if enable {
		cmd := exec.Command("schtasks", "/Create", "/TN", "Ply", "/TR", `"`+exe+`"`, "/SC", "ONLOGON", "/RL", "HIGHEST", "/F")
		tuneCmd(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("автозапуск: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	cmd := exec.Command("schtasks", "/Delete", "/TN", "Ply", "/F")
	tuneCmd(cmd)
	_ = cmd.Run()
	return nil
}
