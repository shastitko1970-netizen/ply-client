//go:build windows

package main

import (
	"os"

	"github.com/jchv/go-webview2"

	"ply/internal/core"
)

func openWebShell() bool {
	d, err := core.EnsureWorker()
	if err != nil {
		return false
	}
	data, err := core.WebViewDir()
	if err != nil {
		return false
	}
	_ = os.Setenv("WEBVIEW2_USER_DATA_FOLDER", data)
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		DataPath:  data,
		WindowOptions: webview2.WindowOptions{
			Title:  core.WindowTitle,
			Width:  960,
			Height: 640,
			IconId: 1,
			Center: true,
		},
	})
	if w == nil {
		return false
	}
	defer w.Destroy()
	w.SetSize(380, 480, webview2.HintMin)
	w.Navigate(d.ShellURL())
	w.Run()
	return true
}
