package core

import (
	"os"
	"path/filepath"
	"strings"
)

func splitPath() string {
	dir, err := DataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "split.txt")
}

func SaveSplit(on bool) error {
	dir, err := DataDir()
	if err != nil {
		return err
	}
	v := "0"
	if on {
		v = "1"
	}
	return os.WriteFile(filepath.Join(dir, "split.txt"), []byte(v+"\n"), 0644)
}

func ReadSplit() bool {
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

func RussiaDirectDomains() []string {
	return []string{
		"geosite:tld-ru",
		"geosite:category-ru",
		"geosite:category-gov-ru",
		"geosite:yandex",
		"geosite:vk",
		"geosite:mailru",
		`regexp:.*\.ru$`,
		`regexp:.*\.xn--p1ai$`,
		`regexp:.*\.su$`,
		"domain:vk.com",
		"domain:userapi.com",
		"domain:vk-cdn.net",
		"domain:vkuser.net",
		"domain:yandex.com",
		"domain:yandex.net",
		"domain:yastatic.net",
		"domain:yandex.cloud",
		"domain:ya.ru",
	}
}

func RussiaDirectIPs() []string {
	return []string{"geoip:ru"}
}
