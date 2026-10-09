//go:build windows

package main

import (
	"os"
	"os/exec"
	"runtime"
	"sync"

	"github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"

	"ply/internal/core"
)

var (
	engMu    sync.Mutex
	shellEng *core.Daemon
)

func setShellEng(d *core.Daemon) {
	engMu.Lock()
	shellEng = d
	engMu.Unlock()
}

func getShellEng() *core.Daemon {
	engMu.Lock()
	defer engMu.Unlock()
	return shellEng
}

func init() {
	runtime.LockOSThread()
	enableHighDPI()
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
	w.SetSize(dip(680), dip(440), webview2.HintMin)
	dressWindow(hwnd)
	setOuterSize(hwnd, dip(800), dip(680))
	core.AttachTrayTo(windows.HWND(hwnd), core.TrayHooks{
		OnConnect: func() {
			go func() {
				eng := getShellEng()
				if eng == nil {
					return
				}
				_, _ = eng.Connect(core.ReadURL(), true)
			}()
		},
		OnDisconnect: func() {
			go func() {
				eng := getShellEng()
				if eng == nil {
					return
				}
				_, _ = eng.Disconnect()
			}()
		},
		OnQuit: func() {
			go func() {
				eng := getShellEng()
				if eng != nil {
					_ = eng.Quit()
				}
				core.RequestQuit()
			}()
		},
	})
	w.SetHtml(bootHTML)
	go func() {
		d, err := core.EnsureWorker()
		if err != nil {
			w.Dispatch(func() {
				w.Eval(`document.getElementById('boot') && (document.getElementById('boot').textContent='Ядро не поднялось. Согласись на права, если Windows спросит.')`)
			})
			return
		}
		setShellEng(d)
		_ = d.YieldTray()
		u := d.ShellURL()
		w.Dispatch(func() { w.Navigate(u) })
	}()
	w.Run()
	return true
}

func openBrowserShell() bool {
	d, err := core.EnsureWorker()
	if err != nil {
		return false
	}
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", d.ShellURL())
	if err := cmd.Start(); err != nil {
		return false
	}
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
			Height: uint(dip(680)),
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
    if (t && t.closest && t.closest('.btns,button,input,textarea,a,.toggle,.power,.field,.sheet,.seg')) return;
    if (t && t.closest && t.closest('.chrome')) return;
    if ((t && t.closest && t.closest('[data-drag]')) || e.clientY <= 40) {
      try { plyWin('drag'); } catch (err) {}
    }
  }, true);
})();`
