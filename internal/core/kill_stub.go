//go:build !windows && !linux

package core

import (
	"fmt"
	"runtime"
)

func EngageKill(xrayPath string) error {
	_ = xrayPath
	return fmt.Errorf("kill-switch на %s пока нет", runtime.GOOS)
}

func ReleaseKill() { clearKillMarker() }

func relaxKill() {}
