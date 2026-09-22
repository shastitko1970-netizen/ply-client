package core

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
)

func DefaultInstallDir() string {
	if runtime.GOOS == "windows" {
		if pf := os.Getenv("ProgramFiles"); pf != "" {
			return filepath.Join(pf, "Ply")
		}
		return `C:\Program Files\Ply`
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return filepath.Join(local, "Ply")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Ply")
}

func LegacyInstallDir() string {
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return filepath.Join(local, "Ply")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "Local", "Ply")
}

func MigrateLegacyData(dest string) {
	old := filepath.Join(LegacyInstallDir(), "data", "url.txt")
	neu := filepath.Join(dest, "data", "url.txt")
	if _, err := os.Stat(neu); err == nil {
		return
	}
	b, err := os.ReadFile(old)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(neu), 0755)
	_ = os.WriteFile(neu, b, 0644)
}

func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	cerr := out.Close()
	if err != nil {
		return err
	}
	return cerr
}
