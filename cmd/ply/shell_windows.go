//go:build windows

package main

import (
	"os"
	"runtime"

	"github.com/jchv/go-webview2"

	"ply/internal/core"
)

func init() {
	runtime.LockOSThread()
}

func openWebShell() bool {
	coInitSTA()
	data, _ := core.WebViewDir()
	if data != "" {
		_ = os.Setenv("WEBVIEW2_USER_DATA_FOLDER", data)
	}
	w := makeView(data)
	if w == nil {
		w = makeView("")
	}
	if w == nil {
		return false
	}
	defer w.Destroy()
	hwnd := uintptr(w.Window())
	_ = w.Bind("plyWin", func(cmd string) error {
		switch cmd {
		case "min":
			winMin(hwnd)
		case "close":
			winClose(hwnd)
		case "drag":
			winDrag(hwnd)
		}
		return nil
	})
	w.Init(dragJS)
	w.SetSize(dip(720), dip(480), webview2.HintMin)
	dressWindow(hwnd)
	setOuterSize(hwnd, dip(800), dip(540))
	w.SetHtml(bootHTML)
	go func() {
		d, err := core.EnsureWorker()
		if err != nil {
			w.Dispatch(func() {
				w.Eval(`document.getElementById('boot') && (document.getElementById('boot').textContent='Ядро не поднялось. Согласись на права, если Windows спросит.')`)
			})
			return
		}
		u := d.ShellURL()
		w.Dispatch(func() { w.Navigate(u) })
	}()
	w.Run()
	return true
}

func makeView(data string) webview2.WebView {
	return webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		DataPath:  data,
		WindowOptions: webview2.WindowOptions{
			Title:  core.WindowTitle,
			Width:  uint(dip(800)),
			Height: uint(dip(540)),
			Center: true,
		},
	})
}

const bootHTML = `<!DOCTYPE html><html><body style="margin:0;background:#0a0a0b;color:#a1a1aa">
<div data-drag style="height:36px;-webkit-app-region:drag;app-region:drag"></div>
<div style="padding:18px 24px;font:italic 500 34px Georgia,'Times New Roman',serif;color:#f4f4f5">Ply</div>
<p id="boot" style="padding:0 24px;font:13px 'Segoe UI',sans-serif">поднимаю ядро…</p>
</body></html>`

const dragJS = `(function(){
  document.addEventListener('mousedown', function(e){
    if (e.button !== 0) return;
    var t = e.target;
    if (t && t.closest && t.closest('.btns,button,input,textarea,a,.toggle,.power,.field')) return;
    if ((t && t.closest && t.closest('.chrome,[data-drag]')) || e.clientY <= 40) {
      try { plyWin('drag'); } catch (err) {}
    }
  }, true);
})();`
