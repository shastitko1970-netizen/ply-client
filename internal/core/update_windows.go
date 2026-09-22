//go:build windows

package core

import (
	"fmt"
	"os/exec"
	"syscall"
)

func RunInstaller(path string) error {
	cmd := exec.Command(path)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    false,
		CreationFlags: 0x00000010, // CREATE_NEW
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("запуск установщика: %w", err)
	}
	return nil
}
