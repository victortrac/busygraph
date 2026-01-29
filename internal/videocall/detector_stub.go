//go:build !darwin && !linux

package videocall

// isCameraActive is a stub for unsupported platforms
func isCameraActive() bool {
	return false
}

// isMicrophoneActive is a stub for unsupported platforms
func isMicrophoneActive() bool {
	return false
}

// getCameraUsers is a stub for unsupported platforms
func getCameraUsers() []string {
	return nil
}

// getMicrophoneUsers is a stub for unsupported platforms
func getMicrophoneUsers() []string {
	return nil
}
