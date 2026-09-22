//go:build windows

package core

import "os/exec"

func prepareWebViewDir(dir string) {
	grant := exec.Command("icacls", dir, "/grant", "*S-1-5-32-545:(OI)(CI)M")
	tuneCmd(grant)
	_ = grant.Run()
	il := exec.Command("icacls", dir, "/setintegritylevel", "(OI)(CI)M")
	tuneCmd(il)
	_ = il.Run()
}
