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
		time.Sleep(400 * time.Millisecond)
		core.BootSaved()
	}()
	core.RunHost(core.TrayHooks{
		OnShow: func() {
			go func() { _ = core.LaunchUI() }()
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
