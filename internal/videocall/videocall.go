package videocall

import (
	"time"
)

// CallState represents the current video call status
type CallState struct {
	InCall           bool      `json:"in_call"`
	CameraActive     bool      `json:"camera_active"`
	MicrophoneActive bool      `json:"microphone_active"`
	CameraUsers      []string  `json:"camera_users"`      // Apps currently using the camera
	MicrophoneUsers  []string  `json:"microphone_users"`  // Apps currently using the microphone
	LastChecked      time.Time `json:"last_checked"`
}

// StateCallback is called when the video call state is updated
type StateCallback func(inCall, cameraActive, micActive bool, app string)

// Detector provides video call detection capabilities
type Detector interface {
	// GetState returns the current call state
	GetState() CallState
	// Start begins periodic polling for call state
	Start(pollInterval time.Duration)
	// Stop halts the polling loop
	Stop()
	// IsInCall returns true if any call indicators are active
	IsInCall() bool
	// SetCallback sets the callback for state updates
	SetCallback(cb StateCallback)
}
