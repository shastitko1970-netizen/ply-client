package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Prefs struct {
	Theme   string `json:"theme"`
	Density string `json:"density"`
	Silent  bool   `json:"silent"`
	Core    string `json:"core"`
	MTU     int    `json:"mtu"`
	Split   bool   `json:"split"`
	Auto    bool   `json:"auto"`
}

func DefaultPrefs() Prefs {
	return Prefs{
		Theme:   "system",
		Density: "compact",
		Silent:  true,
		Core:    "xray",
		MTU:     1400,
		Split:   true,
		Auto:    true,
	}
}

func prefsPath() string {
	dir, err := DataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "prefs.json")
}

func NormalizePrefs(p Prefs) Prefs {
	switch p.Theme {
	case "light", "dark", "system":
	default:
		p.Theme = "system"
	}
	switch p.Density {
	case "compact", "roomy":
	default:
		p.Density = "compact"
	}
	switch p.MTU {
	case 1280, 1400, 1500:
	default:
		p.MTU = 1400
	}
	if p.Core != "xray" {
		p.Core = "xray"
	}
	return p
}

func peekPrefs() (Prefs, bool) {
	var p Prefs
	path := prefsPath()
	if path == "" {
		return p, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return p, false
	}
	if json.Unmarshal(b, &p) != nil {
		return p, false
	}
	return NormalizePrefs(p), true
}

func LoadPrefs() Prefs {
	if p, ok := peekPrefs(); ok {
		return p
	}
	p := DefaultPrefs()
	p.Split = readSplitFile()
	return p
}

func SavePrefs(p Prefs) error {
	p = NormalizePrefs(p)
	dir, err := DataDir()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "prefs.json"), b, 0644); err != nil {
		return err
	}
	v := "0\n"
	if p.Split {
		v = "1\n"
	}
	return os.WriteFile(filepath.Join(dir, "split.txt"), []byte(v), 0644)
}

func EffectiveMTU() int {
	return LoadPrefs().MTU
}

func autostartTarget() string {
	p := LoadPrefs()
	if p.Silent {
		if exe, err := os.Executable(); err == nil {
			return exe
		}
	}
	if ui, err := UIExe(); err == nil {
		return ui
	}
	exe, _ := os.Executable()
	return exe
}

func readSplitFile() bool {
	b, err := os.ReadFile(splitPath())
	if err != nil {
		return true
	}
	s := strings.ToLower(strings.TrimSpace(string(b)))
	if s == "0" || s == "off" || s == "false" || s == "no" {
		return false
	}
	return true
}
