//go:build !windows

package core

func IsAdmin() bool { return true }

func RelaunchElevated() error { return nil }
