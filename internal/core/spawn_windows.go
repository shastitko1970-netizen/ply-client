//go:build windows

package core

import (
	"os/exec"
	"path/filepath"
	"syscall"
)

func startCoreProcess(exe string) error {
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
	cmd := exec.Command(exe)
	cmd.Dir = filepath.Dir(exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    false,
		CreationFlags: 0x00000200 | 0x01000000, // NEW_PROCESS_GROUP | BREAKAWAY_FROM_JOB
	}
	return cmd.Start()
}
