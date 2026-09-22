package core

import "testing"

func TestUnblockFileNoPanic(t *testing.T) {
	UnblockFile("")
	UnblockFile("/tmp/no-such-ply-bin.exe")
	UnblockDir("/tmp/no-such-ply-dir")
	DefendInstallDir("")
}
