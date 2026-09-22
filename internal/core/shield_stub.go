//go:build !windows

package core

func UnblockFile(path string)    { _ = path }
func UnblockDir(dir string)      { _ = dir }
func DefendInstallDir(dir string) { _ = dir }
