//go:build !windows

package core

func SetWinProxy(server string, enable bool) error {
	_, _ = server, enable
	return nil
}

func SetAutoStart(enable bool, exe string) error {
	_, _ = enable, exe
	return nil
}
