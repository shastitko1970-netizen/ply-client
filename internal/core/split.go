package core

import "path/filepath"

func splitPath() string {
	dir, err := DataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "split.txt")
}

func SaveSplit(on bool) error {
	p := LoadPrefs()
	p.Split = on
	return SavePrefs(p)
}

func ReadSplit() bool {
	if p, ok := peekPrefs(); ok {
		return p.Split
	}
	return readSplitFile()
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
