//go:build linux

package videocall

import (
	"os/exec"
	"strings"
)

// isCameraActive checks if any camera device is currently in use on Linux
func isCameraActive() bool {
	// Check if any process has /dev/video* devices open
	videoDevices := []string{
		"/dev/video0",
		"/dev/video1",
		"/dev/video2",
		"/dev/video3",
	}

	for _, dev := range videoDevices {
		cmd := exec.Command("lsof", dev)
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			return true
		}
	}

	return false
}

// isMicrophoneActive checks if any microphone is currently in use on Linux
func isMicrophoneActive() bool {
	// Try PulseAudio first (most common)
	cmd := exec.Command("pactl", "list", "source-outputs")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Source Output #") {
				return true
			}
		}
	}

	// Fallback: check ALSA devices
	cmd = exec.Command("fuser", "-v", "/dev/snd/*")
	output, _ = cmd.CombinedOutput()
	if len(output) > 0 && !strings.Contains(string(output), "No such file") {
		return true
	}

	return false
}

// getCameraUsers returns a list of process names currently using the camera
func getCameraUsers() []string {
	result := make([]string, 0)
	seen := make(map[string]bool)

	videoDevices := []string{
		"/dev/video0",
		"/dev/video1",
		"/dev/video2",
		"/dev/video3",
	}

	for _, dev := range videoDevices {
		cmd := exec.Command("lsof", "-t", dev)
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		// Get PIDs and resolve to process names
		pids := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, pid := range pids {
			if pid == "" {
				continue
			}
			// Get process name from /proc/PID/comm
			commCmd := exec.Command("cat", "/proc/"+pid+"/comm")
			commOutput, err := commCmd.Output()
			if err != nil {
				continue
			}
			procName := strings.TrimSpace(string(commOutput))
			appName := mapLinuxProcessToApp(procName)
			if appName != "" && !seen[appName] {
				seen[appName] = true
				result = append(result, appName)
			}
		}
	}

	return result
}

// getMicrophoneUsers returns a list of process names currently using the microphone
func getMicrophoneUsers() []string {
	result := make([]string, 0)
	seen := make(map[string]bool)

	// Try PulseAudio - parse source-outputs for application.name
	cmd := exec.Command("pactl", "list", "source-outputs")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "application.name") {
				// Extract app name from: application.name = "Firefox"
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					appName := strings.Trim(strings.TrimSpace(parts[1]), "\"")
					mapped := mapLinuxProcessToApp(appName)
					if mapped != "" && !seen[mapped] {
						seen[mapped] = true
						result = append(result, mapped)
					}
				}
			}
		}
	}

	return result
}

// mapLinuxProcessToApp maps Linux process names to user-friendly app names
func mapLinuxProcessToApp(proc string) string {
	procLower := strings.ToLower(proc)

	switch {
	// Browsers
	case strings.Contains(procLower, "brave"):
		return "Brave"
	case strings.Contains(procLower, "chrome") || strings.Contains(procLower, "chromium"):
		return "Chrome"
	case strings.Contains(procLower, "firefox"):
		return "Firefox"
	case strings.Contains(procLower, "edge"):
		return "Edge"

	// Video call apps
	case strings.Contains(procLower, "zoom"):
		return "Zoom"
	case strings.Contains(procLower, "teams"):
		return "Teams"
	case strings.Contains(procLower, "slack"):
		return "Slack"
	case strings.Contains(procLower, "discord"):
		return "Discord"
	case strings.Contains(procLower, "skype"):
		return "Skype"
	case strings.Contains(procLower, "webex"):
		return "Webex"

	// System processes to ignore
	case procLower == "pulseaudio" || procLower == "pipewire" ||
		procLower == "wireplumber" || procLower == "":
		return ""

	default:
		return proc
	}
}
