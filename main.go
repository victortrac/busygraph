package main

import (
	"log"
	"os/exec"
	"runtime"

	"github.com/getlantern/systray"
	"github.com/victortrac/busygraph/internal/hook"
	"github.com/victortrac/busygraph/internal/server"
	"github.com/victortrac/busygraph/internal/tracker"
)

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	log.Println("BusyGraph started")
	systray.SetTitle("BusyGraph")
	systray.SetTooltip("BusyGraph Keystroke Tracker")

	mDashboard := systray.AddMenuItem("Open Dashboard", "View your keystroke stats")
	mQuit := systray.AddMenuItem("Quit", "Quit the application")

	// Initialize tracker
	t := tracker.NewTracker()

	// Start hook in a goroutine
	go func() {
		hook.Start(t)
	}()

	// Start metrics server in a goroutine
	go func() {
		server.Start(":2112", t)
	}()

	// Handle menu items
	go func() {
		for {
			select {
			case <-mDashboard.ClickedCh:
				openBrowser("http://localhost:2112/dashboard")
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "darwin":
		err = exec.Command("open", url).Start()
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	}
	if err != nil {
		log.Printf("Error opening browser: %v", err)
	}
}

func onExit() {
	log.Println("BusyGraph exiting...")
	hook.Stop()
}
