//go:build windows

package core

import (
	"fmt"
	"os/exec"
	"strings"
)

func EngageKill(xrayPath string) error {
	if xrayPath == "" {
		p, err := FindXray()
		if err != nil {
			return fmt.Errorf("kill-switch: %w", err)
		}
		xrayPath = p
	}
	if !killMarkerExists() {
		if err := writeKillMarker(currentFirewallPolicy()); err != nil {
			return err
		}
	}
	if err := runNetsh("advfirewall", "set", "allprofiles", "firewallpolicy", "blockinbound,blockoutbound"); err != nil {
		clearKillMarker()
		return fmt.Errorf("kill-switch: %w", err)
	}
	_ = runNetsh("advfirewall", "firewall", "delete", "rule", "name=PlyKillXray")
	if err := runNetsh("advfirewall", "firewall", "add", "rule", "name=PlyKillXray", "dir=out", "action=allow", "program="+xrayPath, "enable=yes"); err != nil {
		ReleaseKill()
		return fmt.Errorf("kill-switch: %w", err)
	}
	return nil
}

func ReleaseKill() {
	policy := readKillMarker()
	if !strings.Contains(strings.ToLower(policy), "blockinbound") {
		policy = "blockinbound,allowoutbound"
	}
	_ = runNetsh("advfirewall", "firewall", "delete", "rule", "name=PlyKillXray")
	_ = runNetsh("advfirewall", "set", "allprofiles", "firewallpolicy", policy)
	clearKillMarker()
}

func relaxKill() {}

func runNetsh(args ...string) error {
	cmd := exec.Command("netsh", args...)
	tuneCmd(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func currentFirewallPolicy() string {
	cmd := exec.Command("netsh", "advfirewall", "show", "currentprofile")
	tuneCmd(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "blockinbound,allowoutbound"
	}
	for _, line := range strings.Split(string(out), "\n") {
		low := strings.ToLower(line)
		if !strings.Contains(low, "firewall policy") && !strings.Contains(low, "политика") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		last := strings.ToLower(fields[len(fields)-1])
		if strings.Contains(last, "blockinbound") {
			return last
		}
	}
	return "blockinbound,allowoutbound"
}
