//go:build !windows

package core

import (
	"os/exec"
	"path/filepath"
)

func startCoreProcess(exe string) error {
	if !IsAdmin() {
		if err := StartElevated(exe, 0); err == nil {
			return nil
		}
	}
	cmd := exec.Command(exe)
	cmd.Dir = filepath.Dir(exe)
	return cmd.Start()
}

func LaunchUI() error {
	exe, err := UIExe()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe)
	cmd.Dir = filepath.Dir(exe)
	return cmd.Start()
}
