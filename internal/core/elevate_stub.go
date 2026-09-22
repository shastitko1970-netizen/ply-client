//go:build !windows

package core

func IsAdmin() bool { return true }

func RelaunchElevated() error { return nil }

func StartElevated(string, int) error { return nil }
