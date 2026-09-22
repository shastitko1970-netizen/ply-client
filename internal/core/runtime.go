package core

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const LocalPort = 10808

type Session struct {
	Node   *Node
	ExitIP string
}

var (
	watchMu   sync.Mutex
	watchStop chan struct{}
	lastCfg   string
	lastBin   string
)

func AppDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
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

func xrayLogPath() string {
	dir, _ := DataDir()
	return filepath.Join(dir, "xray.log")
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

func FindWintun() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	dir, err := AppDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, "wintun.dll")); err != nil {
		return fmt.Errorf("нет wintun.dll рядом с Ply — туннель без него не встанет")
	}
	return nil
}

func StartXray(bin, cfg string) error {
	logPath := xrayLogPath()
	lf, err := os.Create(logPath)
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, "run", "-c", cfg)
	cmd.Dir = filepath.Dir(bin)
	cmd.Stdout = lf
	cmd.Stderr = lf
	tuneCmd(cmd)
	if err := cmd.Start(); err != nil {
		_ = lf.Close()
		return fmt.Errorf("запуск xray: %w", err)
	}
	go func() {
		_ = cmd.Wait()
		_ = lf.Close()
	}()
	lastBin, lastCfg = bin, cfg
	return os.WriteFile(pidFile(), []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
}

func killXrayProc() {
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
	if runtime.GOOS == "windows" {
		cmd := exec.Command("taskkill", "/F", "/IM", "xray.exe", "/T")
		tuneCmd(cmd)
		_ = cmd.Run()
	}
}

func StopXray() {
	stopWatchdog()
	killXrayProc()
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

func tailLog(n int) string {
	b, err := os.ReadFile(xrayLogPath())
	if err != nil || len(b) == 0 {
		return ""
	}
	s := strings.TrimSpace(string(b))
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func tunFailed(log string) string {
	low := strings.ToLower(log)
	switch {
	case strings.Contains(low, "access is denied"), strings.Contains(low, "elevation"):
		return "нет прав администратора для туннеля"
	case strings.Contains(low, "wintun"):
		return "wintun: " + lastLine(log)
	case strings.Contains(low, "tun") && (strings.Contains(low, "fail") || strings.Contains(low, "error")):
		return lastLine(log)
	}
	return ""
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "\n"); i >= 0 {
		return strings.TrimSpace(s[i+1:])
	}
	return s
}

func ProbeExitIP() string {
	cli := &http.Client{Timeout: 8 * time.Second}
	res, err := cli.Get("https://api.ipify.org")
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 64))
	if err != nil {
		return ""
	}
	ip := strings.TrimSpace(string(b))
	if net.ParseIP(ip) == nil {
		return ""
	}
	return ip
}

func startWatchdog() {
	stopWatchdog()
	ch := make(chan struct{})
	watchMu.Lock()
	watchStop = ch
	watchMu.Unlock()
	go func() {
		t := time.NewTicker(4 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ch:
				return
			case <-t.C:
				alive := PortOpen(LocalPort)
				if runtime.GOOS == "windows" {
					alive = alive && PlyAdapterUp()
				}
				if alive {
					continue
				}
				if lastBin == "" || lastCfg == "" {
					continue
				}
				killXrayProc()
				time.Sleep(200 * time.Millisecond)
				_ = StartXray(lastBin, lastCfg)
				_ = WaitPort(LocalPort, 4*time.Second)
			}
		}
	}()
}

func stopWatchdog() {
	watchMu.Lock()
	if watchStop != nil {
		close(watchStop)
		watchStop = nil
	}
	watchMu.Unlock()
}

func Connect(source string) (*Session, error) {
	if runtime.GOOS == "windows" && !IsAdmin() {
		return nil, fmt.Errorf("для туннеля нужны права администратора")
	}
	if err := FindWintun(); err != nil {
		return nil, err
	}
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
	if runtime.GOOS == "windows" {
		AllowFirewall(xray)
		if exe, e := os.Executable(); e == nil {
			AllowFirewall(exe)
		}
	}
	StopXray()
	time.Sleep(200 * time.Millisecond)
	if err := StartXray(xray, cfgPath); err != nil {
		return nil, err
	}
	if err := WaitPort(LocalPort, 6*time.Second); err != nil {
		msg := tunFailed(tailLog(12))
		StopXray()
		if msg != "" {
			return nil, fmt.Errorf("туннель: %s", msg)
		}
		return nil, fmt.Errorf("%w\n%s", err, tailLog(8))
	}
	if runtime.GOOS == "windows" {
		if !WaitPlyAdapter(8 * time.Second) {
			msg := tunFailed(tailLog(20))
			if msg == "" {
				msg = "адаптер Ply Tunnel не поднялся — Windows останется на Wi‑Fi"
			}
			StopXray()
			return nil, fmt.Errorf("туннель: %s\n%s", msg, tailLog(6))
		}
		PreferAdapterMetric()
		_ = SetWinProxy("", false) // leftover from 1.0/1.1
	} else if msg := tunFailed(tailLog(20)); msg != "" {
		StopXray()
		return nil, fmt.Errorf("туннель: %s", msg)
	}
	_ = SaveURL(source)
	startWatchdog()
	ip := ProbeExitIP()
	if runtime.GOOS == "windows" {
		TrayBalloon("Ply", "VPN включён. Туннель Ply Tunnel.")
	}
	return &Session{Node: n, ExitIP: ip}, nil
}

func Disconnect() error {
	StopXray()
	if runtime.GOOS == "windows" {
		return SetWinProxy("", false)
	}
	return nil
}
