package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Update struct {
	Tag      string
	SetupURL string
	Notes    string
}

func parseVer(s string) [3]int {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	var out [3]int
	parts := strings.Split(s, ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		n, _ := strconv.Atoi(parts[i])
		out[i] = n
	}
	return out
}

func VersionLess(a, b string) bool {
	va, vb := parseVer(a), parseVer(b)
	for i := 0; i < 3; i++ {
		if va[i] < vb[i] {
			return true
		}
		if va[i] > vb[i] {
			return false
		}
	}
	return false
}

func CheckLatest() (*Update, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", GitHubOwner, GitHubRepo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UA)
	req.Header.Set("Accept", "application/vnd.github+json")
	cli := &http.Client{Timeout: 12 * time.Second}
	res, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("github HTTP %d", res.StatusCode)
	}
	var raw struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&raw); err != nil {
		return nil, err
	}
	if !VersionLess(Version, raw.TagName) {
		return nil, nil
	}
	u := &Update{Tag: strings.TrimPrefix(raw.TagName, "v"), Notes: raw.Body}
	for _, a := range raw.Assets {
		switch {
		case strings.EqualFold(a.Name, "PlySetup.exe"):
			u.SetupURL = a.URL
		case u.SetupURL == "" && strings.HasSuffix(strings.ToLower(a.Name), ".exe"):
			u.SetupURL = a.URL
		}
	}
	if u.SetupURL == "" {
		return nil, fmt.Errorf("в релизе %s нет PlySetup.exe", raw.TagName)
	}
	return u, nil
}

func DownloadSetup(url, dest string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UA)
	cli := &http.Client{Timeout: 3 * time.Minute}
	res, err := cli.Do(req)
	if err != nil {
		return fmt.Errorf("скачать установщик: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("скачать установщик: HTTP %d", res.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, io.LimitReader(res.Body, 200<<20))
	cerr := f.Close()
	if err != nil {
		return err
	}
	if cerr != nil {
		return cerr
	}
	fi, err := os.Stat(dest)
	if err != nil {
		return err
	}
	if fi.Size() < 1<<20 {
		_ = os.Remove(dest)
		return fmt.Errorf("установщик битый (слишком маленький)")
	}
	return nil
}
