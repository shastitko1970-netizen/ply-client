//go:build !windows

package core

func RunInstaller(path string) error {
	_ = path
	return nil
}
