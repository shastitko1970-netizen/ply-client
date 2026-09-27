package core

import (
	"os"
	"path/filepath"
	"strings"
)

func killMarkerPath() string {
	dir, err := DataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "kill.on")
}

func writeKillMarker(s string) error {
	p := killMarkerPath()
	if p == "" {
		return os.ErrInvalid
	}
	return os.WriteFile(p, []byte(strings.TrimSpace(s)+"\n"), 0600)
}

func readKillMarker() string {
	b, err := os.ReadFile(killMarkerPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func killMarkerExists() bool {
	p := killMarkerPath()
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

func clearKillMarker() {
	_ = os.Remove(killMarkerPath())
}

func healKill() {
	if killMarkerExists() {
		ReleaseKill()
	}
}
