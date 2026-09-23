package main

import (
	"os"
	"time"

	"ply/internal/core"
)

func main() {
	core.SetAppID()
	if !core.AcquireCoreInstance() {
		os.Exit(0)
	}
	if err := core.StartDaemon(); err != nil {
		os.Exit(1)
	}
	go func() {
		removed := core.RemoveVpnProfile()
		core.BootSaved()
		if removed {
			time.Sleep(800 * time.Millisecond)
			core.TrayBalloon("Ply", "Убрала «Ply» из списка VPN Windows. Он был пустой — отсюда «неверные данные аккаунта». Включай из значка у часов.")
		}
	}()
	core.RunHost(core.TrayHooks{
		OnShow: func() {
			go func() { _ = core.LaunchUI() }()
		},
		OnConnect: func() {
			go core.EngineConnectSaved()
		},
		OnDisconnect: func() {
			go core.EngineDisconnect()
		},
		OnQuit: func() {
			go func() {
				core.EngineDisconnect()
				core.StopDaemon()
				core.RequestQuit()
			}()
		},
	})
	core.EngineDisconnect()
	core.StopDaemon()
}
