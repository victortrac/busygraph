package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/getlantern/systray"
	"github.com/victortrac/busygraph/internal/hook"
	"github.com/victortrac/busygraph/internal/server"
	"github.com/victortrac/busygraph/internal/tracker"
	webview "github.com/webview/webview_go"
)

var isMini = flag.Bool("mini", false, "Start in mini dashboard mode")

func main() {
	flag.Parse()
	if *isMini {
		openQuickStats()
		return
	}
	systray.Run(onReady, onExit)
}

func onReady() {
	log.Println("BusyGraph started")
	systray.SetTitle("BusyGraph")
	systray.SetTooltip("BusyGraph Keystroke Tracker")

	// Clean up any stale lock file from previous run
	lockFile := getMiniLockPath()
	if err := os.Remove(lockFile); err == nil {
		log.Printf("DEBUG: Cleaned up stale lock file: %s", lockFile)
	}

	// Menu items:
	// Quick stats display (non-clickable)
	mKeysToday := systray.AddMenuItem("Keys: -", "Total keystrokes today")
	mKPM := systray.AddMenuItem("KPM: -", "Keystrokes per minute")
	mMouse := systray.AddMenuItem("Mouse: -", "Mouse distance today")
	mKeysToday.Disable()
	mKPM.Disable()
	mMouse.Disable()

	systray.AddSeparator()

	mQuickStats := systray.AddMenuItem("Quick Stats Window", "Open the quick stats popup")
	mDashboard := systray.AddMenuItem("Open Dashboard", "View your keystroke stats in browser")

	systray.AddSeparator()

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

	// Update stats in menu periodically
	go func() {
		updateMenuStats(t, mKeysToday, mKPM, mMouse)
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			updateMenuStats(t, mKeysToday, mKPM, mMouse)
		}
	}()

	// Handle menu item clicks via channels
	go func() {
		for {
			select {
			case <-mQuickStats.ClickedCh:
				log.Println("DEBUG: Quick Stats menu item clicked")
				openQuickStatsWindow()
			case <-mDashboard.ClickedCh:
				log.Println("DEBUG: Open Dashboard menu item clicked")
				openBrowser("http://localhost:2112/dashboard")
			case <-mQuit.ClickedCh:
				log.Println("DEBUG: Quit menu item clicked")
				systray.Quit()
				return
			}
		}
	}()
}

func updateMenuStats(t *tracker.Tracker, mKeys, mKPM, mMouse *systray.MenuItem) {
	stats := t.GetStats("24h")

	// Format keystrokes with comma separator
	mKeys.SetTitle(fmt.Sprintf("Keys: %s", formatNumber(stats.Total)))

	// Show current KPM (avg for the day)
	mKPM.SetTitle(fmt.Sprintf("KPM: %.1f avg, %d max", stats.KPM.Avg, stats.KPM.Max))

	// Mouse distance (convert pixels to meters, assuming 96 DPI)
	meters := stats.Mouse.Distance / (96 / 0.0254)
	mMouse.SetTitle(fmt.Sprintf("Mouse: %.1fm, %d clicks", meters, stats.Mouse.ClicksLeft+stats.Mouse.ClicksRight))
}

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%d,%03d", n/1000, n%1000)
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

func getMiniLockPath() string {
	return filepath.Join(os.TempDir(), "busygraph-mini.lock")
}

func openQuickStatsWindow() {
	log.Println("DEBUG: openQuickStatsWindow called")
	// Check if mini window is already open
	lockFile := getMiniLockPath()
	log.Printf("DEBUG: Checking lock file: %s", lockFile)
	if _, err := os.Stat(lockFile); err == nil {
		// Window exists, try to focus it
		log.Println("DEBUG: Lock file exists, focusing existing window")
		focusMiniWindow()
		return
	}
	log.Println("DEBUG: No lock file, opening new window")

	exe, err := os.Executable()
	if err != nil {
		log.Printf("Error getting executable: %v", err)
		return
	}
	log.Printf("DEBUG: Starting %s --mini", exe)
	exec.Command(exe, "--mini").Start()
}

func focusMiniWindow() {
	if runtime.GOOS == "darwin" {
		// Use AppleScript to bring the window to front
		script := `tell application "System Events" to set frontmost of (first process whose name contains "busygraph") to true`
		exec.Command("osascript", "-e", script).Run()
	}
}

func openQuickStats() {
	// Create lock file
	lockFile := getMiniLockPath()
	f, err := os.Create(lockFile)
	if err != nil {
		log.Printf("Error creating lock file: %v", err)
	} else {
		f.Close()
	}
	defer os.Remove(lockFile)

	debug := false
	w := webview.New(debug)
	defer w.Destroy()

	w.SetTitle("BusyGraph Quick Stats")
	w.SetSize(400, 450, webview.HintFixed)
	w.Navigate("http://localhost:2112/mini")
	w.Run()
}

func onExit() {
	log.Println("BusyGraph exiting...")
	hook.Stop()
}
