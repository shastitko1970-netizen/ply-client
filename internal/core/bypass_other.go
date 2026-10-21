//go:build !windows

package core

func presetFolders(family string) []string {
	_ = family
	return nil
}
