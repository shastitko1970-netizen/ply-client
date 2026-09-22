//go:build !windows

package core

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func IsAdmin() bool {
	return os.Geteuid() == 0
}

func StartElevated(exe string, _ int) error {
	if exe == "" {
		return fmt.Errorf("нет ядра")
	}
	if runtime.GOOS == "darwin" {
		esc := strings.ReplaceAll(strings.ReplaceAll(exe, `\`, `\\`), `"`, `\"`)
		script := `do shell script "` + esc + `" with administrator privileges`
		cmd := exec.Command("osascript", "-e", script)
		return cmd.Start()
	}
	for _, helper := range []string{"pkexec", "sudo"} {
		if p, err := exec.LookPath(helper); err == nil {
			cmd := exec.Command(p, exe)
			return cmd.Start()
		}
	}
	return fmt.Errorf("нужен root: pkexec или sudo")
}

func RelaunchElevated() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return StartElevated(exe, 1)
}
