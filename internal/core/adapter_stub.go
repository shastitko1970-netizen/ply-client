//go:build !windows

package core

import "time"

func PlyAdapterUp() bool { return true }

func WaitPlyAdapter(d time.Duration) bool {
	_ = d
	return true
}

func AllowFirewall(exe string) { _ = exe }

func PreferAdapterMetric() {}
