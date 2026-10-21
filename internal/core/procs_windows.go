//go:build windows

package core

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func ListProcs() (running, installed, folders []ProcHit) {
	seenRun := map[string]struct{}{}
	seenExe := map[string]struct{}{}
	seenDir := map[string]struct{}{}
	var gameFolders []ProcHit
	var otherFolders []ProcHit

	addFolder := func(name, path string, game bool) {
		rule := folderRule(path)
		if rule == "" {
			return
		}
		key := strings.ToLower(rule)
		if _, ok := seenDir[key]; ok {
			return
		}
		if coveredBy(key, seenDir) {
			return
		}
		seenDir[key] = struct{}{}
		hit := ProcHit{Name: name, Path: rule, Kind: "folder"}
		if game {
			if len(gameFolders) < 24 {
				gameFolders = append(gameFolders, hit)
			}
			return
		}
		if len(otherFolders) < 24 {
			otherFolders = append(otherFolders, hit)
		}
	}

	for _, dir := range knownGameDirs() {
		addFolder(filepath.Base(dir), liftInstallDir(dir), true)
	}

	for _, root := range []struct {
		k registry.Key
		p string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
	} {
		walkUninstall(root.k, root.p, addFolder, func(name, path string) {
			if len(installed) >= 60 {
				return
			}
			key := strings.ToLower(path)
			if _, ok := seenExe[key]; ok {
				return
			}
			seenExe[key] = struct{}{}
			installed = append(installed, ProcHit{Name: name, Path: slashPath(path), Kind: "exe"})
		})
	}

	folders = append(gameFolders, otherFolders...)
	running = listRunning(seenRun)
	return running, installed, folders
}

func listRunning(seen map[string]struct{}) []ProcHit {
	var out []ProcHit
	for _, full := range runningImagePaths() {
		if len(out) >= 80 {
			break
		}
		if full == "" || skipProcess(full) {
			continue
		}
		hit := runningHit(full)
		key := strings.ToLower(hit.Path)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, hit)
	}
	return out
}

func runningImagePaths() []string {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	snapProc := kernel32.NewProc("CreateToolhelp32Snapshot")
	first := kernel32.NewProc("Process32FirstW")
	next := kernel32.NewProc("Process32NextW")
	open := kernel32.NewProc("OpenProcess")
	query := kernel32.NewProc("QueryFullProcessImageNameW")
	closeH := kernel32.NewProc("CloseHandle")

	const snapProcess = 0x00000002
	handle, _, _ := snapProc.Call(snapProcess, 0)
	if handle == 0 || handle == ^uintptr(0) {
		return nil
	}
	defer closeH.Call(handle)

	type processEntry32 struct {
		Size            uint32
		Usage           uint32
		ProcessID       uint32
		DefaultHeapID   uintptr
		ModuleID        uint32
		Threads         uint32
		ParentProcessID uint32
		PriClassBase    int32
		Flags           uint32
		ExeFile         [260]uint16
	}
	var pe processEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	r, _, _ := first.Call(handle, uintptr(unsafe.Pointer(&pe)))
	var out []string
	for r != 0 {
		if len(out) >= 400 {
			break
		}
		if full := imagePath(open, query, closeH, pe.ProcessID); full != "" {
			out = append(out, full)
		}
		pe.Size = uint32(unsafe.Sizeof(pe))
		r, _, _ = next.Call(handle, uintptr(unsafe.Pointer(&pe)))
	}
	return out
}

func imagePath(open, query, closeH *windows.LazyProc, pid uint32) string {
	if pid == 0 || pid == 4 {
		return ""
	}
	const queryLimited = 0x1000
	h, _, _ := open.Call(queryLimited, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer closeH.Call(h)
	var buf [1024]uint16
	size := uint32(len(buf))
	r, _, _ := query.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:])
}

func skipProcess(full string) bool {
	low := strings.ToLower(full)
	if strings.Contains(low, `\windows\system32\`) || strings.Contains(low, `\windows\syswow64\`) || strings.Contains(low, `\windows\winsxs\`) {
		return true
	}
	base := strings.ToLower(filepath.Base(full))
	switch base {
	case "ply.exe", "plycore.exe", "xray.exe", "explorer.exe":
		return true
	}
	return false
}

func walkUninstall(root registry.Key, path string, addFolder func(name, path string, game bool), addExe func(name, path string)) {
	k, err := registry.OpenKey(root, path, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if err != nil {
		return
	}
	defer k.Close()
	names, _ := k.ReadSubKeyNames(400)
	for _, name := range names {
		sk, err := registry.OpenKey(k, name, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		disp, _, _ := sk.GetStringValue("DisplayName")
		loc, _, _ := sk.GetStringValue("InstallLocation")
		icon, _, _ := sk.GetStringValue("DisplayIcon")
		sys, _, sysErr := sk.GetIntegerValue("SystemComponent")
		sk.Close()
		if disp == "" || (sysErr == nil && sys == 1) {
			continue
		}
		low := strings.ToLower(disp)
		if strings.Contains(low, "security update") || strings.Contains(low, "update for") || strings.HasPrefix(low, "kb") {
			continue
		}
		exe := iconToExe(icon)
		dir := ""
		if loc != "" {
			loc = strings.Trim(loc, `"`)
			if st, err := os.Stat(loc); err == nil && st.IsDir() {
				dir = loc
			}
		}
		if dir == "" && exe != "" {
			dir = filepath.Dir(exe)
		}
		game := gameish(disp + " " + dir)
		if dir != "" && (game || loc != "") {
			addFolder(disp, liftInstallDir(dir), game)
		}
		if exe != "" {
			addExe(disp, exe)
		}
	}
}

func knownGameDirs() []string {
	names := append(append([]string{}, hoyoDirNames...), blizzardDirNames...)
	var out []string
	seen := map[string]struct{}{}
	for _, root := range installRoots() {
		for _, name := range names {
			p := filepath.Join(root, name)
			st, err := os.Stat(p)
			if err != nil || !st.IsDir() {
				continue
			}
			key := strings.ToLower(p)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, p)
		}
	}
	return out
}

var hoyoDirNames = []string{
	"HoYoPlay", "miHoYo", "miHoYo Launcher", "Cognosphere",
	"Genshin Impact", "Honkai Star Rail", "Star Rail",
	"Zenless Zone Zero", "ZenlessZoneZero", "ZenlessZoneZero Game",
}

var blizzardDirNames = []string{
	"Battle.net", "Blizzard", "Blizzard Entertainment", "World of Warcraft",
}

func installRoots() []string {
	seen := map[string]struct{}{}
	var roots []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		p = strings.TrimRight(p, `/\`)
		if p == "" {
			return
		}
		key := strings.ToLower(p)
		if _, ok := seen[key]; ok {
			return
		}
		st, err := os.Stat(p)
		if err != nil || !st.IsDir() {
			return
		}
		seen[key] = struct{}{}
		roots = append(roots, p)
	}
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LOCALAPPDATA", "ProgramData"} {
		add(os.Getenv(env))
	}
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return roots
	}
	for i := uint(0); i < 26; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		p, err := windows.UTF16PtrFromString(root)
		if err != nil || windows.GetDriveType(p) != windows.DRIVE_FIXED {
			continue
		}
		add(root)
		add(filepath.Join(root, "Program Files"))
		add(filepath.Join(root, "Program Files (x86)"))
		add(filepath.Join(root, "Games"))
		add(filepath.Join(root, "ProgramData"))
	}
	return roots
}

func iconToExe(icon string) string {
	icon = strings.TrimSpace(strings.Trim(icon, `"`))
	if icon == "" {
		return ""
	}
	if i := strings.LastIndex(icon, ","); i > 2 {
		if _, err := strconv.Atoi(strings.TrimSpace(icon[i+1:])); err == nil {
			icon = strings.TrimSpace(icon[:i])
		}
	}
	if !strings.HasSuffix(strings.ToLower(icon), ".exe") {
		return ""
	}
	if st, err := os.Stat(icon); err != nil || st.IsDir() {
		return ""
	}
	return icon
}

func folderRule(path string) string {
	path = strings.TrimSpace(strings.Trim(path, `"`))
	if path == "" {
		return ""
	}
	path = slashPath(path)
	path = strings.TrimRight(path, "/")
	if path == "" || strings.Contains(path, "..") {
		return ""
	}
	return path + "/"
}

func slashPath(path string) string {
	return strings.ReplaceAll(path, `\`, "/")
}

func gameish(s string) bool {
	return presetFamily(s) != ""
}

func coveredBy(path string, seen map[string]struct{}) bool {
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	for have := range seen {
		if have != path && strings.HasPrefix(path, have) {
			return true
		}
	}
	return false
}
