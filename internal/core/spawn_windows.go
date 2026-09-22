//go:build windows

package core

import (
	"os/exec"
	"path/filepath"
	"syscall"
)

func startCoreProcess(exe string) error {
	if !IsAdmin() {
		return StartElevated(exe, 0)
	}
	cmd := exec.Command(exe)
	cmd.Dir = filepath.Dir(exe)
	tuneCmd(cmd)
	return cmd.Start()
}

func LaunchUI() error {
	exe, err := UIExe()
	if err != nil {
		return err
	}
	if IsAdmin() {
		if err := startUnelevated(exe); err == nil {
			return nil
		}
	}
	cmd := exec.Command(exe)
	cmd.Dir = filepath.Dir(exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    false,
		CreationFlags: 0x00000200, // NEW_PROCESS_GROUP
	}
	return cmd.Start()
}

func startUnelevated(exe string) error {
	cmd := exec.Command("explorer.exe", exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    false,
		CreationFlags: 0x00000200, // NEW_PROCESS_GROUP
	}
	return cmd.Start()
}
