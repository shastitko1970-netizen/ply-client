//go:build windows

package main

import (
	"os"
	"path/filepath"

	"github.com/jchv/go-webview2"

	"ply/internal/core"
)

func openWebShell() bool {
	d, err := core.EnsureWorker()
	if err != nil {
		return false
	}
	data := ""
	if dir, e := core.DataDir(); e == nil {
		data = filepath.Join(dir, "webview")
		_ = os.MkdirAll(data, 0755)
	}
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
