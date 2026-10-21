//go:build windows

package core

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func init() {
	bypassCaseFn = windowsCanonicalBypass
}

func windowsCanonicalBypass(s string) string {
	if strings.Contains(s, "/") {
		trail := strings.HasSuffix(s, "/")
		c := diskCasePath(strings.TrimRight(s, "/"))
		if c == "" {
			return ""
		}
		if trail {
			c = strings.TrimRight(c, "/") + "/"
		}
		return c
	}
	if real := resolveProcName(s); real != "" {
		return real
	}
	return ""
}

func diskCasePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	win := strings.ReplaceAll(path, "/", `\`)
	p16, err := windows.UTF16PtrFromString(win)
	if err != nil {
		return ""
	}
	h, err := windows.CreateFile(p16, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, 1024)
	n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), 0)
	if n >= uint32(len(buf)) {
		buf = make([]uint16, n+1)
		n, err = windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), 0)
	}
	if err != nil || n == 0 {
		return ""
	}
	if int(n) > len(buf) {
		n = uint32(len(buf))
	}
	got := windows.UTF16ToString(buf[:n])
	switch {
	case strings.HasPrefix(got, `\\?\UNC\`):
		got = `\\` + strings.TrimPrefix(got, `\\?\UNC\`)
	case strings.HasPrefix(got, `\\?\`):
		got = strings.TrimPrefix(got, `\\?\`)
	}
	return upperDrive(strings.ReplaceAll(got, `\`, "/"))
}

var (
	procCaseMu sync.Mutex
	procCaseAt time.Time
	procCase   map[string]string
)

func resolveProcName(s string) string {
	key := strings.ToLower(exeSuffix(s))
	key = strings.TrimSuffix(key, ".exe")
	procCaseMu.Lock()
	defer procCaseMu.Unlock()
	if procCase == nil || time.Since(procCaseAt) > 2*time.Second {
		procCase = snapshotProcCases()
		procCaseAt = time.Now()
	}
	if real, ok := procCase[key]; ok {
		return real
	}
	return ""
}

func snapshotProcCases() map[string]string {
	out := map[string]string{}
	add := func(base string) {
		base = exeSuffix(filepath.Base(strings.ReplaceAll(base, `/`, `\`)))
		if base == "" || strings.EqualFold(base, ".") {
			return
		}
		key := strings.ToLower(strings.TrimSuffix(strings.ToLower(base), ".exe"))
		if _, ok := out[key]; ok {
			return
		}
		for _, known := range append(append([]string{}, HoyoProcesses...), BlizzardProcesses...) {
			if strings.EqualFold(known, base) {
				base = known
				break
			}
		}
		out[key] = base
	}
	for _, full := range runningImagePaths() {
		add(full)
	}
	for _, dir := range knownGameDirs() {
		scanExeDir(dir, add)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() && versionComponent(e.Name()) {
				scanExeDir(filepath.Join(dir, e.Name()), add)
			}
		}
	}
	return out
}

func scanExeDir(dir string, add func(string)) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".exe") {
			add(e.Name())
		}
	}
}

func presetFolders(family string) []string {
	seen := map[string]struct{}{}
	var found []string
	add := func(path string) {
		rule := folderRule(liftInstallDir(path))
		if rule == "" {
			return
		}
		if c := diskCasePath(strings.TrimRight(rule, "/")); c != "" {
			rule = folderRule(c)
		}
		found = append(found, rule)
	}
	for _, dir := range knownGameDirs() {
		if presetFamily(dir) == family {
			add(dir)
		}
	}
	for _, root := range []struct {
		k registry.Key
		p string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
	} {
		walkUninstall(root.k, root.p, func(name, path string, _ bool) {
			if presetFamily(name+" "+path) == family {
				add(path)
			}
		}, func(string, string) {})
	}
	// Shorter parents win so a versioned child is not a second rule.
	for i := 0; i < len(found); i++ {
		for j := i + 1; j < len(found); j++ {
			if len(found[j]) < len(found[i]) {
				found[i], found[j] = found[j], found[i]
			}
		}
	}
	var out []string
	for _, rule := range found {
		key := strings.ToLower(rule)
		if _, ok := seen[key]; ok || coveredBy(key, seen) {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, rule)
	}
	return out
}
