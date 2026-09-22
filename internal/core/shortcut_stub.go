//go:build !windows

package core

func CreateShortcut(link, target, workdir, desc string) error {
	_, _, _, _ = link, target, workdir, desc
	return nil
}

func DesktopDir() string         { return "" }
func StartMenuDir() string       { return "" }
func CommonStartMenuDir() string { return "" }
