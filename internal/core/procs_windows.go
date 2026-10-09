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
		addFolder(filepath.Base(dir), dir, true)
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
	var out []ProcHit
	for r != 0 {
		if len(out) >= 80 {
			break
		}
		full := imagePath(open, query, closeH, pe.ProcessID)
		if full != "" && !skipProcess(full) {
			key := strings.ToLower(full)
			if _, ok := seen[key]; !ok {
				seen[key] = struct{}{}
				out = append(out, ProcHit{
					Name: filepath.Base(full),
					Path: slashPath(full),
					Kind: "running",
				})
			}
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
			addFolder(disp, dir, game)
		}
		if exe != "" {
			addExe(disp, exe)
		}
	}
}

func knownGameDirs() []string {
	var roots []string
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LOCALAPPDATA", "ProgramData"} {
		if v := os.Getenv(env); v != "" {
			roots = append(roots, v)
		}
	}
	names := []string{
		"HoYoPlay", "miHoYo", "miHoYo Launcher", "Cognosphere",
		"Genshin Impact", "Honkai Star Rail", "Star Rail",
		"Zenless Zone Zero", "ZenlessZoneZero",
	}
	var out []string
	for _, root := range roots {
		for _, name := range names {
			p := filepath.Join(root, name)
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				out = append(out, p)
			}
		}
	}
	return out
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
	s = strings.ToLower(s)
	for _, k := range []string{"hoyo", "mihoyo", "genshin", "star rail", "zenless", "cognosphere", "honkai"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
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
