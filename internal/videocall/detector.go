package videocall

import (
	"sync"
	"time"
)

// detector is the main implementation that combines all detection methods
type detector struct {
	mu       sync.RWMutex
	state    CallState
	stopCh   chan struct{}
	running  bool
	callback StateCallback
}

// NewDetector creates a new video call detector
func NewDetector() Detector {
	return &detector{
		stopCh: make(chan struct{}),
	}
}

// SetCallback sets the callback for state updates
func (d *detector) SetCallback(cb StateCallback) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.callback = cb
}

// GetState returns the current call state
func (d *detector) GetState() CallState {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.state
}

// IsInCall returns true if any call indicators are active
func (d *detector) IsInCall() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.state.InCall
}

// Start begins periodic polling for call state
func (d *detector) Start(pollInterval time.Duration) {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return
	}
	d.running = true
	d.stopCh = make(chan struct{})
	d.mu.Unlock()

	// Do an initial check
	d.update()

	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-d.stopCh:
				return
			case <-ticker.C:
				d.update()
			}
		}
	}()
}

// Stop halts the polling loop
func (d *detector) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.running {
		close(d.stopCh)
		d.running = false
	}
}

// update refreshes the call state by checking all sources
func (d *detector) update() {
	// Check camera and microphone status using OS APIs (source of truth)
	cameraActive := isCameraActive()
	micActive := isMicrophoneActive()

	// Always check for call apps (browser URLs, native call processes)
	// This detects calls even before camera/mic is enabled
	callApps := getCameraUsers() // This now checks browser URLs and native apps

	// Determine if we're in a call:
	// - A video call app/browser tab is detected (browser on meet.google.com, Zoom CptHost, etc.)
	// - OR camera is active (strong signal)
	// - OR mic is active AND a video call app is using it
	inCall := len(callApps) > 0 || cameraActive

	// Build the users lists
	var cameraUsers, micUsers []string
	if cameraActive || len(callApps) > 0 {
		cameraUsers = callApps
	}
	if micActive || len(callApps) > 0 {
		micUsers = callApps
	}

	// Get the primary app name for tracking
	app := ""
	if len(callApps) > 0 {
		app = callApps[0]
	}

	d.mu.Lock()
	d.state = CallState{
		InCall:           inCall,
		CameraActive:     cameraActive,
		MicrophoneActive: micActive,
		CameraUsers:      cameraUsers,
		MicrophoneUsers:  micUsers,
		LastChecked:      time.Now(),
	}
	cb := d.callback
	d.mu.Unlock()

	// Call the callback to persist state
	if cb != nil {
		cb(inCall, cameraActive, micActive, app)
	}
}

// videoCallApps is the list of known video call applications
var knownVideoCallApps = map[string]bool{
	"Zoom":     true,
	"Teams":    true,
	"Slack":    true,
	"Discord":  true,
	"FaceTime": true,
	"Skype":    true,
	"Webex":    true,
	// Browsers (could be Google Meet, etc.)
	"Brave":   true,
	"Chrome":  true,
	"Firefox": true,
	"Safari":  true,
	"Arc":     true,
	"Edge":    true,
}

// hasVideoCallApp checks if any of the apps in the list is a known video call app
func hasVideoCallApp(apps []string) bool {
	for _, app := range apps {
		if knownVideoCallApps[app] {
			return true
		}
	}
	return false
}
