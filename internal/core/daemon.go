package core

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"ply/internal/core/shell"
)

type Snapshot struct {
	Version  string     `json:"version"`
	Platform string     `json:"platform"`
	Admin    bool       `json:"admin"`
	Live     bool       `json:"live"`
	Busy     bool       `json:"busy"`
	Status   string     `json:"status"`
	Error    string     `json:"error"`
	Detail   string     `json:"detail"`
	ExitIP   string     `json:"exitIP"`
	URL      string     `json:"url"`
	Split    bool       `json:"split"`
	Auto     bool       `json:"auto"`
	Prefs    Prefs      `json:"prefs"`
	Cores    []CoreSlot `json:"cores,omitempty"`
	Node     *Node      `json:"node,omitempty"`
	Update   *Update    `json:"update,omitempty"`
	UpdNote  string     `json:"updNote,omitempty"`
}

type coreFile struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
	PID   int    `json:"pid"`
}

var (
	daemonMu   sync.Mutex
	daemonSnap Snapshot
	daemonSrv  *http.Server
	daemonTok  string
	daemonLn   net.Listener
)

func coreFilePath() string {
	dir, err := DataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "core.json")
}

func writeCoreFile(port int, token string) error {
	p := coreFilePath()
	if p == "" {
		return fmt.Errorf("нет папки данных")
	}
	b, err := json.Marshal(coreFile{Port: port, Token: token, PID: os.Getpid()})
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0600)
}

func readCoreFile() (coreFile, error) {
	var c coreFile
	p := coreFilePath()
	if p == "" {
		return c, fmt.Errorf("нет папки данных")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	if c.Port <= 0 || c.Token == "" {
		return c, fmt.Errorf("битый core.json")
	}
	return c, nil
}

func randomToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func setSnap(fn func(*Snapshot)) {
	daemonMu.Lock()
	defer daemonMu.Unlock()
	fn(&daemonSnap)
}

func currentSnap() Snapshot {
	daemonMu.Lock()
	defer daemonMu.Unlock()
	s := daemonSnap
	s.Version = Version
	s.Platform = runtime.GOOS
	s.Admin = IsAdmin()
	s.URL = ReadURL()
	p := LoadPrefs()
	s.Split = p.Split
	s.Auto = p.Auto
	s.Prefs = p
	s.Cores = CoreCatalog()
	if s.Status == "" {
		s.Status = "ожидание"
	}
	return s
}

func daemonMux(token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", 405)
			return
		}
		writeJSON(w, currentSnap())
	})
	mux.HandleFunc("/v1/connect", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		var body struct {
			URL  string `json:"url"`
			Auto bool   `json:"auto"`
		}
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body)
		go daemonConnect(body.URL, body.Auto)
		writeJSON(w, currentSnap())
	})
	mux.HandleFunc("/v1/disconnect", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		EngineDisconnect()
		writeJSON(w, currentSnap())
	})
	mux.HandleFunc("/v1/split", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		var body struct {
			On bool `json:"on"`
		}
		_ = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body)
		_ = SaveSplit(body.On)
		setSnap(func(s *Snapshot) { s.Split = body.On })
		if currentSnap().Live {
			go daemonConnect(ReadURL(), false)
		}
		writeJSON(w, currentSnap())
	})
	mux.HandleFunc("/v1/autostart", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		var body struct {
			On bool `json:"on"`
		}
		_ = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body)
		exe := autostartTarget()
		if err := SetAutoStart(body.On, exe); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		p := LoadPrefs()
		p.Auto = body.On
		_ = SavePrefs(p)
		setSnap(func(s *Snapshot) { s.Auto = body.On })
		writeJSON(w, currentSnap())
	})
	mux.HandleFunc("/v1/prefs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		var body Prefs
		_ = json.NewDecoder(io.LimitReader(r.Body, 8192)).Decode(&body)
		prev := LoadPrefs()
		if err := UseCore(body.Core); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		body.Core = "xray"
		if err := SavePrefs(body); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		next := LoadPrefs()
		if next.Auto != prev.Auto || next.Silent != prev.Silent {
			_ = SetAutoStart(next.Auto, autostartTarget())
		}
		setSnap(func(s *Snapshot) {
			s.Split = next.Split
			s.Auto = next.Auto
			s.Prefs = next
		})
		if currentSnap().Live && (next.MTU != prev.MTU || next.Split != prev.Split) {
			go daemonConnect(ReadURL(), false)
		}
		writeJSON(w, currentSnap())
	})
	mux.HandleFunc("/v1/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		go daemonConnect(ReadURL(), false)
		writeJSON(w, currentSnap())
	})
	mux.HandleFunc("/v1/update/check", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			http.Error(w, "method", 405)
			return
		}
		daemonCheckUpdate(true)
		writeJSON(w, currentSnap())
	})
	mux.HandleFunc("/v1/update/apply", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		go daemonApplyUpdate()
		writeJSON(w, currentSnap())
	})
	mux.HandleFunc("/v1/quit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		writeJSON(w, map[string]string{"ok": "1"})
		go func() {
			time.Sleep(80 * time.Millisecond)
			_ = Disconnect()
			StopDaemon()
			RequestQuit()
			time.Sleep(200 * time.Millisecond)
			os.Exit(0)
		}()
	})
	mux.HandleFunc("/ui", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(shell.HTML)
	})
	mux.HandleFunc("/fonts/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/fonts/")
		b, ok := shell.Font(name)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "font/ttf")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write(b)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ui" || strings.HasPrefix(r.URL.Path, "/fonts/") {
			mux.ServeHTTP(w, r)
			return
		}
		got := r.Header.Get("Authorization")
		want := "Bearer " + token
		if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func EngineDisconnect() {
	_ = Disconnect()
	setSnap(func(s *Snapshot) {
		s.Live = false
		s.Busy = false
		s.Status = "ожидание"
		s.Error = ""
		s.Detail = ""
		s.ExitIP = ""
		s.Node = nil
	})
}

func EngineConnectSaved() {
	u := strings.TrimSpace(ReadURL())
	if u == "" {
		TrayBalloon("Ply", "Ключа нет. Открываю окно — вставь ссылку.")
		_ = LaunchUI()
		return
	}
	daemonMu.Lock()
	live := daemonSnap.Live
	busy := daemonSnap.Busy
	daemonMu.Unlock()
	if busy {
		return
	}
	if live {
		TrayBalloon("Ply", "Туннель уже включён.")
		return
	}
	daemonConnect(u, false)
}

func BootSaved() {
	if !LoadPrefs().Auto {
		return
	}
	u := strings.TrimSpace(ReadURL())
	if u == "" {
		return
	}
	daemonConnect(u, false)
}

var connectMu sync.Mutex

func daemonConnect(src string, auto bool) {
	connectMu.Lock()
	defer connectMu.Unlock()
	src = StripSubHash(src)
	setSnap(func(s *Snapshot) {
		s.Busy = true
		s.Error = ""
		s.Status = "включаю"
	})
	sess, err := Connect(src)
	if err != nil {
		setSnap(func(s *Snapshot) {
			s.Busy = false
			s.Live = false
			s.Error = err.Error()
			s.Status = "ошибка"
			s.Detail = ""
			s.ExitIP = ""
			s.Node = nil
		})
		return
	}
	detail := fmt.Sprintf("%s  %s:%d", sess.Node.Label(), sess.Node.Host, sess.Node.Port)
	status := sess.Node.Label()
	if sess.Split {
		status = "РФ напрямую"
	}
	setSnap(func(s *Snapshot) {
		s.Busy = false
		s.Live = true
		s.Error = ""
		s.Status = status
		s.Detail = detail
		s.ExitIP = sess.ExitIP
		s.Node = sess.Node
		s.Split = sess.Split
		if auto {
			s.Auto = true
		}
	})
	if auto {
		p := LoadPrefs()
		p.Auto = true
		_ = SavePrefs(p)
		_ = SetAutoStart(true, autostartTarget())
	}
	go daemonCheckUpdate(false)
}

func daemonCheckUpdate(manual bool) {
	upd, err := CheckLatest()
	setSnap(func(s *Snapshot) {
		if err != nil {
			if manual {
				s.UpdNote = err.Error()
			}
			return
		}
		if upd == nil {
			s.Update = nil
			if manual {
				s.UpdNote = "уже свежая v" + Version
			}
			return
		}
		s.Update = upd
		s.UpdNote = ""
	})
}

func daemonApplyUpdate() {
	daemonMu.Lock()
	upd := daemonSnap.Update
	daemonMu.Unlock()
	if upd == nil || upd.SetupURL == "" {
		setSnap(func(s *Snapshot) { s.UpdNote = "нет ссылки на установщик" })
		return
	}
	setSnap(func(s *Snapshot) { s.UpdNote = "скачиваю установщик…" })
	dest := filepath.Join(os.TempDir(), "PlySetup-"+upd.Tag+".exe")
	if err := DownloadSetup(upd.SetupURL, dest); err != nil {
		setSnap(func(s *Snapshot) { s.UpdNote = err.Error() })
		return
	}
	setSnap(func(s *Snapshot) { s.UpdNote = "запускаю установщик — Ply закроется" })
	_ = Disconnect()
	if err := RunInstaller(dest); err != nil {
		setSnap(func(s *Snapshot) { s.UpdNote = err.Error() })
		return
	}
	RemoveTray()
	StopDaemon()
	os.Exit(0)
}

func StartDaemon() error {
	tok, err := randomToken()
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := writeCoreFile(port, tok); err != nil {
		_ = ln.Close()
		return err
	}
	srv := &http.Server{
		Handler:           daemonMux(tok),
		ReadHeaderTimeout: 4 * time.Second,
	}
	daemonMu.Lock()
	daemonSrv = srv
	daemonTok = tok
	daemonLn = ln
	daemonSnap = Snapshot{
		Version: Version,
		Admin:   IsAdmin(),
		Status:  "ожидание",
		Split:   ReadSplit(),
		Auto:    true,
		URL:     ReadURL(),
	}
	daemonMu.Unlock()
	go func() { _ = srv.Serve(ln) }()
	go func() {
		time.Sleep(1500 * time.Millisecond)
		if RemoveVpnProfile() {
			TrayBalloon("Ply", "Убрала «Ply» из списка VPN Windows. Он был пустой — отсюда «неверные данные аккаунта». Включай из значка у часов.")
		}
	}()
	go func() {
		time.Sleep(1200 * time.Millisecond)
		daemonCheckUpdate(false)
	}()
	return nil
}

func StopDaemon() {
	daemonMu.Lock()
	srv := daemonSrv
	ln := daemonLn
	daemonSrv = nil
	daemonLn = nil
	daemonMu.Unlock()
	if srv != nil {
		_ = srv.Close()
	}
	if ln != nil {
		_ = ln.Close()
	}
	_ = os.Remove(coreFilePath())
}
