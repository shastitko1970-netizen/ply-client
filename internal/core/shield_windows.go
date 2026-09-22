//go:build windows

package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func UnblockFile(path string) {
	if path == "" {
		return
	}
	_ = os.Remove(path + ":Zone.Identifier")
}

func UnblockDir(dir string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".exe" || ext == ".dll" || ext == ".bat" {
			UnblockFile(filepath.Join(dir, name))
		}
	}
}

func DefendInstallDir(dir string) {
	if dir == "" {
		return
	}
	script := "Add-MpPreference -ExclusionPath '" + dir + "' -ErrorAction SilentlyContinue;" +
		"Add-MpPreference -ExclusionProcess 'xray.exe' -ErrorAction SilentlyContinue;" +
		"Add-MpPreference -ExclusionProcess 'PlyCore.exe' -ErrorAction SilentlyContinue"
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	tuneCmd(cmd)
	_ = cmd.Run()
}
