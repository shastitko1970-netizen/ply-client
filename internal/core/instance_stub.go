//go:build !windows

package core

func SetAppID() {}

func AcquireInstance() bool { return true }

func ActivateExisting() {}
