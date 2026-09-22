package core

import (
	"os"
	"path/filepath"
	"runtime"
)

// WebViewDir is the Edge WebView2 user-data folder.
// Must be user-writable at medium integrity — never Program Files.
func WebViewDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if runtime.GOOS == "windows" {
			base = filepath.Join(home, "AppData", "Local")
		} else {
			base = filepath.Join(home, ".local", "share")
		}
	}
	d := filepath.Join(base, "Ply", "webview")
	if err := os.MkdirAll(d, 0755); err != nil {
		return "", err
	}
	prepareWebViewDir(d)
	return d, nil
}
