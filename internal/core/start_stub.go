//go:build !windows

package core

import "os/exec"

func tuneCmd(cmd *exec.Cmd) {}
