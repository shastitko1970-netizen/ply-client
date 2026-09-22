package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type Daemon struct {
	base  string
	token string
	cli   *http.Client
}

func newDaemon(port int, token string) *Daemon {
	return &Daemon{
		base:  fmt.Sprintf("http://127.0.0.1:%d", port),
		token: token,
		cli:   &http.Client{Timeout: 12 * time.Second},
	}
}

func (d *Daemon) do(method, path string, body any) (*Snapshot, error) {
	var rdr *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, d.base+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+d.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := d.cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("ядро не пустило")
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("ядро HTTP %d", res.StatusCode)
	}
	var snap Snapshot
	if err := json.NewDecoder(res.Body).Decode(&snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

func (d *Daemon) State() (*Snapshot, error) {
	return d.do(http.MethodGet, "/v1/state", nil)
}

func (d *Daemon) Connect(url string, auto bool) (*Snapshot, error) {
	return d.do(http.MethodPost, "/v1/connect", map[string]any{"url": url, "auto": auto})
}

func (d *Daemon) Disconnect() (*Snapshot, error) {
	return d.do(http.MethodPost, "/v1/disconnect", map[string]any{})
}

func (d *Daemon) SetSplit(on bool) (*Snapshot, error) {
	return d.do(http.MethodPost, "/v1/split", map[string]any{"on": on})
}

func (d *Daemon) SetAuto(on bool) (*Snapshot, error) {
	return d.do(http.MethodPost, "/v1/autostart", map[string]any{"on": on})
}

func (d *Daemon) Refresh() (*Snapshot, error) {
	return d.do(http.MethodPost, "/v1/refresh", map[string]any{})
}

func (d *Daemon) CheckUpdate() (*Snapshot, error) {
	return d.do(http.MethodPost, "/v1/update/check", map[string]any{})
}

func (d *Daemon) ApplyUpdate() (*Snapshot, error) {
	return d.do(http.MethodPost, "/v1/update/apply", map[string]any{})
}

func (d *Daemon) Quit() error {
	_, err := d.do(http.MethodPost, "/v1/quit", map[string]any{})
	return err
}

func TryAttach() *Daemon {
	c, err := readCoreFile()
	if err != nil {
		return nil
	}
	d := newDaemon(c.Port, c.Token)
	st, err := d.State()
	if err != nil || st == nil {
		return nil
	}
	return d
}

func CoreExe() (string, error) {
	dir, err := AppDir()
	if err != nil {
		return "", err
	}
	names := []string{CoreExeName}
	if runtime.GOOS != "windows" {
		names = append(names, "PlyCore")
	}
	for _, n := range names {
		p := filepath.Join(dir, n)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("нет %s рядом с Ply", CoreExeName)
}

func UIExe() (string, error) {
	dir, err := AppDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, UIExeName)
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	p = filepath.Join(dir, "Ply")
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("нет Ply.exe")
}

func EnsureWorker() (*Daemon, error) {
	if d := TryAttach(); d != nil {
		return d, nil
	}
	exe, err := CoreExe()
	if err != nil {
		return nil, err
	}
	if err := startCoreProcess(exe); err != nil {
		return nil, fmt.Errorf("запуск ядра: %w", err)
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if d := TryAttach(); d != nil {
			return d, nil
		}
		time.Sleep(80 * time.Millisecond)
	}
	return nil, fmt.Errorf("ядро не ответило")
}
