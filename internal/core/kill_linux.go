//go:build linux

package core

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func EngageKill(xrayPath string) error {
	_ = xrayPath
	pid := xrayPID()
	if pid <= 0 {
		return fmt.Errorf("kill-switch: xray ещё не запущен")
	}
	if _, err := exec.LookPath("iptables"); err != nil {
		return fmt.Errorf("kill-switch: нет iptables")
	}
	script := fmt.Sprintf(`
iptables -N PLY_KILL 2>/dev/null || true
iptables -F PLY_KILL
iptables -A PLY_KILL -o lo -j ACCEPT
iptables -A PLY_KILL -m owner --pid-owner %d -j ACCEPT
iptables -A PLY_KILL -j DROP
iptables -C OUTPUT -j PLY_KILL 2>/dev/null || iptables -I OUTPUT 1 -j PLY_KILL
`, pid)
	if err := runIPTables(script); err != nil {
		return err
	}
	return writeKillMarker("linux")
}

func relaxKill() {
	if !LoadPrefs().Kill {
		return
	}
	ip := net.ParseIP(strings.TrimSpace(lastServerIP))
	if ip == nil || ip.To4() == nil {
		return
	}
	ip4 := ip.To4().String()
	script := fmt.Sprintf(`
iptables -N PLY_KILL 2>/dev/null || true
iptables -F PLY_KILL
iptables -A PLY_KILL -o lo -j ACCEPT
iptables -A PLY_KILL -d %s -j ACCEPT
iptables -A PLY_KILL -j DROP
iptables -C OUTPUT -j PLY_KILL 2>/dev/null || iptables -I OUTPUT 1 -j PLY_KILL
`, ip4)
	_ = runIPTables(script)
	_ = writeKillMarker("linux")
}

func ReleaseKill() {
	if _, err := exec.LookPath("iptables"); err == nil {
		_ = runIPTables(`
iptables -D OUTPUT -j PLY_KILL 2>/dev/null || true
iptables -F PLY_KILL 2>/dev/null || true
iptables -X PLY_KILL 2>/dev/null || true
`)
	}
	clearKillMarker()
}

func runIPTables(script string) error {
	cmd := exec.Command("sh", "-c", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("kill-switch: %s", msg)
	}
	return nil
}

func xrayPID() int {
	b, err := os.ReadFile(pidFile())
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}
