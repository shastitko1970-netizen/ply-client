//go:build !windows

package main

import (
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"ply/internal/core"
)

func main() {
	core.SetAppID()
	if !core.AcquireInstance() {
		core.ActivateExisting()
		os.Exit(0)
	}
	d, err := core.EnsureWorker()
	if err != nil {
		os.Stderr.WriteString("Ply: " + err.Error() + "\n")
		os.Exit(1)
	}
	url := d.ShellURL()
	cmd := appWindow(url)
	if cmd != nil {
		if err := cmd.Start(); err == nil && cmd.Process != nil {
			go func() {
				_ = cmd.Wait()
				os.Exit(0)
			}()
		}
	} else {
		openFallback(url)
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func appWindow(page string) *exec.Cmd {
	data, _ := core.WebViewDir()
	if data == "" {
		data = filepath.Join(os.TempDir(), "ply-chrome")
	}
	_ = os.MkdirAll(data, 0755)
	args := []string{"--app=" + page, "--user-data-dir=" + data, "--window-size=800,540", "--disable-extensions"}
	bins := chromeBins()
	for _, b := range bins {
		if p, err := exec.LookPath(b); err == nil {
			return exec.Command(p, args...)
		}
		if _, err := os.Stat(b); err == nil {
			return exec.Command(b, args...)
		}
	}
	if runtime.GOOS == "darwin" {
		chrome := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
		if _, err := os.Stat(chrome); err == nil {
			return exec.Command(chrome, args...)
		}
		edge := "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"
		if _, err := os.Stat(edge); err == nil {
			return exec.Command(edge, args...)
		}
	}
	return nil
}

func chromeBins() []string {
	return []string{
		"google-chrome", "google-chrome-stable", "chromium", "chromium-browser",
		"microsoft-edge", "microsoft-edge-stable", "brave-browser", "brave",
	}
}

func openFallback(page string) {
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("open", page).Start()
	default:
		_ = exec.Command("xdg-open", page).Start()
	}
	time.Sleep(300 * time.Millisecond)
}
