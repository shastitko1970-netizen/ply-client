package core

import (
	"regexp"
	"strings"
)

// bypassCaseFn rewrites one already slash-normalized token to its on-disk
// casing. An empty result keeps the token. Windows replaces this in init.
var bypassCaseFn = func(string) string { return "" }

// directResolverFn overrides DirectResolver in tests.
var directResolverFn func() string

func DirectResolver() string {
	if directResolverFn != nil {
		return directResolverFn()
	}
	return platformDirectResolver()
}

// Xray compares the process base name with slices.Contains after stripping a
// lowercase ".exe" only, and compares folder prefixes as exact strings.
// Bare names keep the letters the user typed; the suffix is normalized so the
// strip works. Paths keep their case aside from the drive letter.
func applyBypassCase(s string) string {
	if strings.Contains(s, "/") {
		s = upperDrive(s)
	} else {
		s = exeSuffix(s)
	}
	if bypassCaseFn != nil {
		if c := bypassCaseFn(s); c != "" {
			s = c
		}
	}
	return s
}

func exeSuffix(s string) string {
	if len(s) >= 4 && strings.EqualFold(s[len(s)-4:], ".exe") {
		return s[:len(s)-4] + ".exe"
	}
	return s
}

func upperDrive(s string) string {
	if len(s) >= 2 && s[1] == ':' && s[0] >= 'a' && s[0] <= 'z' {
		s = string(s[0]-32) + s[1:]
	}
	return s
}

var (
	pureVersion  = regexp.MustCompile(`^\d+(?:\.\d+){1,}$`)
	namedVersion = regexp.MustCompile(`(?i)^[a-z][a-z0-9 ._-]*\.\d[\d.]*$`)
)

func versionComponent(seg string) bool {
	if seg == "" {
		return false
	}
	return pureVersion.MatchString(seg) || namedVersion.MatchString(seg)
}

// stableParent is the install folder above the first versioned component.
// HoYoPlay\<version>\HYP.exe and Agent.<build>\Agent.exe both collapse.
// A path with no versioned component, or one that would become just a drive,
// is left alone.
func stableParent(path string) (string, bool) {
	p := strings.ReplaceAll(path, `\`, "/")
	p = strings.TrimRight(p, "/")
	if p == "" || strings.Contains(p, "..") {
		return "", false
	}
	parts := strings.Split(p, "/")
	idx := -1
	for i, seg := range parts {
		if i == 0 || seg == "" {
			continue
		}
		if versionComponent(seg) {
			idx = i
			break
		}
	}
	if idx < 2 {
		return "", false
	}
	return strings.Join(parts[:idx], "/") + "/", true
}

func runningHit(full string) ProcHit {
	full = strings.ReplaceAll(full, `\`, "/")
	if folder, ok := stableParent(full); ok {
		return ProcHit{Name: pathBase(folder), Path: folder, Kind: "folder"}
	}
	return ProcHit{Name: pathBase(full), Path: full, Kind: "running"}
}

func pathBase(p string) string {
	p = strings.TrimRight(strings.ReplaceAll(p, `\`, "/"), "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func liftInstallDir(dir string) string {
	if stable, ok := stableParent(dir); ok {
		return strings.TrimRight(stable, "/")
	}
	return dir
}

// Preset process names use the on-disk spelling Xray compares against.
// launcher.exe is generic, but it is the HoYoPlay/miHoYo launcher binary.
var HoyoProcesses = []string{
	"HYP.exe",
	"HYPHelper.exe",
	"launcher.exe",
	"ZenlessZoneZero.exe",
	"HYUpdater.exe",
}

var BlizzardProcesses = []string{
	"Battle.net.exe",
	"Agent.exe",
	"Wow.exe",
	"WowClassic.exe",
	"WowT.exe",
}

// Parent domains only. Subdomains (autopatch, nap API, CDN hostnames under
// these zones) match Xray's domain: rule. Public CDNs are not listed.
var HoyoDomains = []string{
	"domain:zenlesszonezero.com",
	"domain:hoyoverse.com",
	"domain:mihoyo.com",
	"domain:hoyolab.com",
}

var BlizzardDomains = []string{
	"domain:battle.net",
	"domain:blizzard.com",
}

// BlizzardIPv4 is the AS-BLIZZARD (AS57976) IRR aggregate set.
// Snapshot 2026-10-21. More-specifics announced in BGP sit inside these.
var BlizzardIPv4 = []string{
	"4.14.106.208/28",
	"5.42.160.0/19",
	"24.105.0.0/18",
	"37.244.0.0/18",
	"59.153.40.0/22",
	"64.111.160.0/20",
	"64.224.0.0/21",
	"64.224.24.0/21",
	"64.227.216.0/21",
	"64.227.228.0/22",
	"64.227.232.0/21",
	"66.40.176.0/20",
	"103.4.114.0/23",
	"103.198.32.0/23",
	"110.45.208.0/24",
	"137.221.64.0/18",
	"150.116.9.0/24",
	"158.115.192.0/19",
	"185.22.80.0/22",
	"185.34.104.0/22",
	"185.60.112.0/22",
	"198.74.32.0/21",
	"198.74.40.0/23",
	"202.9.66.0/23",
	"209.249.12.0/24",
}

func presetFamily(s string) string {
	s = strings.ToLower(s)
	switch {
	case strings.Contains(s, "hoyo"),
		strings.Contains(s, "mihoyo"),
		strings.Contains(s, "genshin"),
		strings.Contains(s, "star rail"),
		strings.Contains(s, "zenless"),
		strings.Contains(s, "cognosphere"),
		strings.Contains(s, "honkai"):
		return "hoyo"
	case strings.Contains(s, "blizzard"),
		strings.Contains(s, "battle.net"),
		strings.Contains(s, "battlenet"),
		strings.Contains(s, "warcraft"):
		return "blizzard"
	default:
		return ""
	}
}

func mergeBypass(base, extra []string) []string {
	out := append([]string{}, base...)
	seen := map[string]struct{}{}
	for _, s := range out {
		seen[strings.ToLower(s)] = struct{}{}
	}
	for _, s := range normalizeBypass(extra, 128) {
		low := strings.ToLower(s)
		if _, ok := seen[low]; ok {
			continue
		}
		seen[low] = struct{}{}
		out = append(out, s)
	}
	return out
}

func appendPresets(bypass []string, hoyo, blizzard bool) (procs, domains, ips []string) {
	var extra []string
	if hoyo {
		extra = append(extra, HoyoProcesses...)
		extra = append(extra, presetFolders("hoyo")...)
		domains = append(domains, HoyoDomains...)
	}
	if blizzard {
		extra = append(extra, BlizzardProcesses...)
		extra = append(extra, presetFolders("blizzard")...)
		domains = append(domains, BlizzardDomains...)
		ips = append(ips, BlizzardIPv4...)
	}
	return mergeBypass(bypass, extra), domains, ips
}
