//go:build !windows

package core

func RunHost(hooks TrayHooks) {
	_ = hooks
	select {}
}
