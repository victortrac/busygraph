package hook

import (
	"log"

	gohook "github.com/robotn/gohook"
	"github.com/victortrac/busygraph/internal/tracker"
)

// Start starts the global key hook
func Start(t *tracker.Tracker) {
	log.Println("Starting global key hook...")
	evChan := gohook.Start()
	defer gohook.End()

	for ev := range evChan {
		if ev.Kind == gohook.KeyDown { // key press
			// log.Println("Key pressed") // Debugging, can be noisy
			key := gohook.RawcodetoKeychar(ev.Rawcode)
			switch key {
			case "\r", "\n":
				key = "[ENTER]"
			case "\t":
				key = "[TAB]"
			case "\b":
				key = "[BACKSPACE]"
			case " ":
				key = "[SPACE]"
			case "\x1b":
				key = "[ESC]"
			case "":
				continue
			}

			// Filter out other control characters
			if len(key) == 1 && key[0] < 32 {
				continue
			}

			t.Increment(key)
		} else if ev.Kind == gohook.MouseMove || ev.Kind == gohook.MouseDrag {
			t.TrackMouseMove(ev.X, ev.Y)
		} else if ev.Kind == gohook.MouseDown {
			// Button 1 = Left, 2 = Right (usually)
			if ev.Button == 1 {
				t.TrackMouseClick("left")
			} else if ev.Button == 2 {
				t.TrackMouseClick("right")
			}
		} else if ev.Kind == gohook.MouseWheel {
			t.TrackMouseScroll(int16(ev.Rotation)) // Rotation is usually amount
		}
	}
}

// Stop stops the global key hook
func Stop() {
	gohook.End()
}
