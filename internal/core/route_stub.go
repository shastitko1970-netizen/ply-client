//go:build !windows

package core

func DefaultViaPly() bool { return true }

func ApplyTunRoutes(serverIP string) error {
	_ = serverIP
	return nil
}

func RestoreTunRoutes() {}

func RegisterVpnProfile(serverIP string) { _ = serverIP }

func RemoveVpnProfile() bool { return false }
