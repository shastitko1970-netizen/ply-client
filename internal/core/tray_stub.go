//go:build !windows

package core

type TrayHooks struct {
	OnShow       func()
	OnConnect    func()
	OnDisconnect func()
	OnQuit       func()
	Invalidate   func()
}

func AttachTray(hooks TrayHooks) bool { return false }
func RemoveTray()                     {}
func HideToTray()                     {}
func ShowFromTray()                   {}
func RequestQuit()                    {}
func TrayBalloon(title, msg string)   {}
func FindPlyHWND() uintptr            { return 0 }
