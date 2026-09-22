package core

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const LocalPort = 10808

func AppDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

func DefaultInstallDir() string {
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return filepath.Join(local, "Ply")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "Local", "Ply")
}

func DataDir() (string, error) {
	dir, err := AppDir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(dir, "data")
	return d, os.MkdirAll(d, 0755)
}

func SaveURL(u string) error {
	dir, err := DataDir()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "url.txt"), []byte(strings.TrimSpace(u)+"\n"), 0644)
}

func ReadURL() string {
	dir, err := DataDir()
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(dir, "url.txt"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func pidFile() string {
	dir, _ := DataDir()
	return filepath.Join(dir, "xray.pid")
}

func FindXray() (string, error) {
	if p := os.Getenv("PLY_XRAY"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	dir, err := AppDir()
	if err != nil {
		return "", err
	}
	for _, p := range []string{
		filepath.Join(dir, "xray.exe"),
		filepath.Join(dir, "xray"),
		filepath.Join(dir, "bin", "xray.exe"),
	} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("не найден xray.exe рядом с Ply")
}

func StartXray(bin, cfg string) error {
	cmd := exec.Command(bin, "run", "-c", cfg)
	cmd.Dir = filepath.Dir(bin)
	cmd.Stdout = nil
	cmd.Stderr = nil
	tuneCmd(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("запуск xray: %w", err)
	}
	return os.WriteFile(pidFile(), []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
}

func StopXray() {
	b, err := os.ReadFile(pidFile())
	if err == nil {
		pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
		if pid > 0 {
			p, err := os.FindProcess(pid)
			if err == nil {
				_ = p.Kill()
			}
		}
	}
	_ = os.Remove(pidFile())
}

func PortOpen(port int) bool {
	c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func WaitPort(port int, d time.Duration) error {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if PortOpen(port) {
			return nil
		}
		time.Sleep(80 * time.Millisecond)
	}
	return fmt.Errorf("ядро не открыло порт")
}

func Connect(source string) (*Node, error) {
	n, err := Resolve(source)
	if err != nil {
		return nil, err
	}
	cfg, err := RenderXray(n, LocalPort)
	if err != nil {
		return nil, err
	}
	dir, err := DataDir()
	if err != nil {
		return nil, err
	}
	cfgPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfgPath, cfg, 0644); err != nil {
		return nil, err
	}
	xray, err := FindXray()
	if err != nil {
		return nil, err
	}
	StopXray()
	time.Sleep(150 * time.Millisecond)
	if err := StartXray(xray, cfgPath); err != nil {
		return nil, err
	}
	if err := WaitPort(LocalPort, 5*time.Second); err != nil {
		return nil, err
	}
	if runtime.GOOS == "windows" {
		if err := SetWinProxy(fmt.Sprintf("127.0.0.1:%d", LocalPort), true); err != nil {
			return nil, fmt.Errorf("прокси Windows: %w", err)
		}
	}
	_ = SaveURL(source)
	return n, nil
}

func Disconnect() error {
	StopXray()
	if runtime.GOOS == "windows" {
		return SetWinProxy("", false)
	}
	return nil
}
