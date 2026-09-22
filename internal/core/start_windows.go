//go:build windows

package core

import (
	"os/exec"
	"syscall"
)

func tuneCmd(cmd *exec.Cmd) {
	const (
		createNoWindow        = 0x08000000
		createNewProcessGroup = 0x00000200
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow | createNewProcessGroup,
	}
}
