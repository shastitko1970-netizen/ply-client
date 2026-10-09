//go:build !windows

package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func ListProcs() (running, installed, folders []ProcHit) {
	switch runtime.GOOS {
	case "linux":
		return linuxProcs(), nil, nil
	case "darwin":
		return darwinProcs(), nil, nil
	default:
		return nil, nil, nil
	}
}

func linuxProcs() []ProcHit {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []ProcHit
	for _, e := range entries {
		if len(out) >= 80 {
			break
		}
		name := e.Name()
		if name == "" || name[0] < '0' || name[0] > '9' {
			continue
		}
		target, err := os.Readlink("/proc/" + name + "/exe")
		if err != nil || target == "" || strings.HasPrefix(target, "[") {
			continue
		}
		target = strings.Split(target, " (deleted)")[0]
		low := strings.ToLower(target)
		base := strings.ToLower(filepath.Base(target))
		if base == "ply" || base == "plycore" || base == "xray" {
			continue
		}
		if strings.Contains(low, "/proc/") || strings.HasPrefix(low, "/sys/") {
			continue
		}
		key := strings.ToLower(target)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, ProcHit{Name: filepath.Base(target), Path: target, Kind: "running"})
	}
	return out
}

func darwinProcs() []ProcHit {
	outb, err := exec.Command("ps", "-ax", "-o", "comm=").Output()
	if err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []ProcHit
	for _, line := range strings.Split(string(outb), "\n") {
		if len(out) >= 80 {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[") {
			continue
		}
		base := filepath.Base(line)
		low := strings.ToLower(base)
		if low == "ply" || low == "plycore" || low == "xray" || low == "ps" {
			continue
		}
		if _, ok := seen[low]; ok {
			continue
		}
		seen[low] = struct{}{}
		kindPath := line
		if !strings.Contains(line, "/") {
			kindPath = ""
		}
		out = append(out, ProcHit{Name: base, Path: kindPath, Kind: "running"})
	}
	return out
}
