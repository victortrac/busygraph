//go:build linux

package hook

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	evdev "github.com/holoplot/go-evdev"
	"github.com/victortrac/busygraph/internal/tracker"
)

var (
	mu      sync.Mutex
	devices []*evdev.InputDevice
	wg      sync.WaitGroup

	// Virtual cursor position for relative mouse → absolute coordinate conversion.
	cursorX, cursorY int16
)

// keycodeMap maps evdev key codes to the string labels used by the tracker.
// Modifiers are intentionally excluded (matching gohook behavior where they
// return an empty string and get skipped).
var keycodeMap = map[evdev.EvCode]string{
	evdev.KEY_A: "a", evdev.KEY_B: "b", evdev.KEY_C: "c", evdev.KEY_D: "d",
	evdev.KEY_E: "e", evdev.KEY_F: "f", evdev.KEY_G: "g", evdev.KEY_H: "h",
	evdev.KEY_I: "i", evdev.KEY_J: "j", evdev.KEY_K: "k", evdev.KEY_L: "l",
	evdev.KEY_M: "m", evdev.KEY_N: "n", evdev.KEY_O: "o", evdev.KEY_P: "p",
	evdev.KEY_Q: "q", evdev.KEY_R: "r", evdev.KEY_S: "s", evdev.KEY_T: "t",
	evdev.KEY_U: "u", evdev.KEY_V: "v", evdev.KEY_W: "w", evdev.KEY_X: "x",
	evdev.KEY_Y: "y", evdev.KEY_Z: "z",

	evdev.KEY_1: "1", evdev.KEY_2: "2", evdev.KEY_3: "3", evdev.KEY_4: "4",
	evdev.KEY_5: "5", evdev.KEY_6: "6", evdev.KEY_7: "7", evdev.KEY_8: "8",
	evdev.KEY_9: "9", evdev.KEY_0: "0",

	evdev.KEY_MINUS:      "-",
	evdev.KEY_EQUAL:      "=",
	evdev.KEY_LEFTBRACE:  "[",
	evdev.KEY_RIGHTBRACE: "]",
	evdev.KEY_SEMICOLON:  ";",
	evdev.KEY_APOSTROPHE: "'",
	evdev.KEY_GRAVE:      "`",
	evdev.KEY_BACKSLASH:  "\\",
	evdev.KEY_COMMA:      ",",
	evdev.KEY_DOT:        ".",
	evdev.KEY_SLASH:      "/",

	evdev.KEY_ENTER:     "[ENTER]",
	evdev.KEY_TAB:       "[TAB]",
	evdev.KEY_BACKSPACE: "[BACKSPACE]",
	evdev.KEY_SPACE:     "[SPACE]",
	evdev.KEY_ESC:       "[ESC]",

	evdev.KEY_F1: "[F1]", evdev.KEY_F2: "[F2]", evdev.KEY_F3: "[F3]",
	evdev.KEY_F4: "[F4]", evdev.KEY_F5: "[F5]", evdev.KEY_F6: "[F6]",
	evdev.KEY_F7: "[F7]", evdev.KEY_F8: "[F8]", evdev.KEY_F9: "[F9]",
	evdev.KEY_F10: "[F10]", evdev.KEY_F11: "[F11]", evdev.KEY_F12: "[F12]",

	evdev.KEY_INSERT:   "[INSERT]",
	evdev.KEY_DELETE:   "[DELETE]",
	evdev.KEY_HOME:     "[HOME]",
	evdev.KEY_END:      "[END]",
	evdev.KEY_PAGEUP:   "[PAGEUP]",
	evdev.KEY_PAGEDOWN: "[PAGEDOWN]",
	evdev.KEY_UP:       "[UP]",
	evdev.KEY_DOWN:     "[DOWN]",
	evdev.KEY_LEFT:     "[LEFT]",
	evdev.KEY_RIGHT:    "[RIGHT]",
}

type deviceKind int

const (
	kindKeyboard deviceKind = 1 << iota
	kindMouse
)

// Start opens all keyboard and mouse evdev devices and begins tracking input.
func Start(t *tracker.Tracker) {
	log.Println("Starting evdev input capture...")

	matches, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		log.Fatalf("Failed to list /dev/input/event* devices: %v", err)
	}

	var opened int
	for _, path := range matches {
		dev, err := evdev.Open(path)
		if err != nil {
			if os.IsPermission(err) {
				log.Fatalf("Permission denied opening %s. Add your user to the 'input' group:\n  sudo usermod -aG input $USER\nthen log out and back in.", path)
			}
			continue // skip devices we can't open
		}

		kind := classifyDevice(dev)
		if kind == 0 {
			dev.Close()
			continue
		}

		name, _ := dev.Name()
		log.Printf("Opened %s: %s (keyboard=%v mouse=%v)",
			path, name, kind&kindKeyboard != 0, kind&kindMouse != 0)

		mu.Lock()
		devices = append(devices, dev)
		mu.Unlock()

		wg.Add(1)
		go readLoop(dev, kind, t)
		opened++
	}

	if opened == 0 {
		log.Fatalf("No usable input devices found. Make sure you have permission to read /dev/input/event* devices.\n  sudo usermod -aG input $USER")
	}

	// Block until all device goroutines exit (i.e. Stop() is called).
	wg.Wait()
}

// Stop closes all open devices, which causes ReadOne() to return an error
// and the goroutines to exit.
func Stop() {
	mu.Lock()
	defer mu.Unlock()

	for _, dev := range devices {
		dev.Close()
	}
	devices = nil
}

// classifyDevice checks capabilities to determine whether dev is a keyboard,
// mouse, or both. Returns 0 if neither.
func classifyDevice(dev *evdev.InputDevice) deviceKind {
	var kind deviceKind

	capableTypes := dev.CapableTypes()
	hasType := func(t evdev.EvType) bool {
		for _, ct := range capableTypes {
			if ct == t {
				return true
			}
		}
		return false
	}

	if hasType(evdev.EV_KEY) {
		codes := dev.CapableEvents(evdev.EV_KEY)
		codeSet := make(map[evdev.EvCode]bool, len(codes))
		for _, c := range codes {
			codeSet[c] = true
		}

		// Keyboard: has letter keys
		if codeSet[evdev.KEY_A] {
			kind |= kindKeyboard
		}
		// Mouse: has mouse buttons
		if codeSet[evdev.BTN_LEFT] && hasType(evdev.EV_REL) {
			kind |= kindMouse
		}
	}

	return kind
}

// readLoop reads events from a single device until it is closed.
func readLoop(dev *evdev.InputDevice, kind deviceKind, t *tracker.Tracker) {
	defer wg.Done()

	// Per-SYN-frame accumulators for relative mouse movement.
	var dx, dy int32

	for {
		ev, err := dev.ReadOne()
		if err != nil {
			return // device closed or error → exit goroutine
		}

		switch ev.Type {
		case evdev.EV_KEY:
			if ev.Value != 1 { // only key press, not repeat (2) or release (0)
				continue
			}

			if kind&kindMouse != 0 {
				switch ev.Code {
				case evdev.BTN_LEFT:
					t.TrackMouseClick("left")
					continue
				case evdev.BTN_RIGHT:
					t.TrackMouseClick("right")
					continue
				}
			}

			if kind&kindKeyboard != 0 {
				if label, ok := keycodeMap[ev.Code]; ok {
					t.Increment(label)
				}
			}

		case evdev.EV_REL:
			if kind&kindMouse == 0 {
				continue
			}
			switch ev.Code {
			case evdev.REL_X:
				dx += ev.Value
			case evdev.REL_Y:
				dy += ev.Value
			case evdev.REL_WHEEL:
				t.TrackMouseScroll(int16(ev.Value))
			}

		case evdev.EV_SYN:
			// SYN_REPORT marks the end of an input frame.
			if ev.Code == 0 && (dx != 0 || dy != 0) {
				newX := clampInt16(int32(cursorX) + dx)
				newY := clampInt16(int32(cursorY) + dy)
				cursorX = newX
				cursorY = newY
				t.TrackMouseMove(newX, newY)
				dx, dy = 0, 0
			}
		}
	}
}

func clampInt16(v int32) int16 {
	switch {
	case v > 32767:
		// Wrap around to avoid getting stuck at the boundary.
		return int16(v - 65536)
	case v < -32768:
		return int16(v + 65536)
	default:
		return int16(v)
	}
}

func init() {
	// Sanity-check: if /dev/input doesn't exist, give a helpful message.
	if _, err := os.Stat("/dev/input"); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "busygraph: /dev/input not found — evdev is not available on this system")
		os.Exit(1)
	}
}
