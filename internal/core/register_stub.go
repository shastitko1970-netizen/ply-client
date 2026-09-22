//go:build !windows

package core

func InstalledVersion() string { return "" }

func WriteVersionFile(dir string) error { return nil }

func StopPlyProcesses() {}

func RegisterApp(dir string) error {
	_ = dir
	return nil
}

func WriteUninstall(dir string) error {
	_ = dir
	return nil
}

func NotifyShell() {}

func RemoveLegacyStartFolder() {}

func SetLogonTask(enable bool, exe string) error {
	_, _ = enable, exe
	return nil
}
