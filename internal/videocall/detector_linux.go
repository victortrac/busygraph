//go:build linux

package videocall

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// isCameraActive checks if any camera device is currently in use on Linux.
func isCameraActive() bool {
	// Method 1: PipeWire — check for active video capture streams.
	// On modern Linux (Arch, Fedora, Ubuntu 22.04+), browsers access cameras
	// through the PipeWire camera portal rather than opening /dev/video*
	// directly, so we need to ask PipeWire if any client is capturing video.
	cmd := exec.Command("pw-dump")
	if output, err := cmd.Output(); err == nil {
		// "Stream/Input/Video" is the media.class for an active video capture
		// stream (e.g. a browser's WebRTC camera feed). It only appears in
		// pw-dump output when a client is actively capturing.
		if strings.Contains(string(output), `"Stream/Input/Video"`) {
			return true
		}
	}

	// Method 2: Scan /proc for processes with /dev/video* fds open.
	// Covers non-PipeWire systems and native apps that open V4L2 directly.
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, entry := range entries {
		pid := entry.Name()
		if !entry.IsDir() || pid[0] < '0' || pid[0] > '9' {
			continue
		}
		fdDir := filepath.Join("/proc", pid, "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			if strings.HasPrefix(target, "/dev/video") {
				// Ignore system daemons that hold the device without streaming.
				comm := readProcComm(pid)
				if comm == "udevd" || comm == "systemd-udevd" {
					continue
				}
				return true
			}
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

// getCameraUsers returns a list of apps currently in a video call.
// It checks both camera device usage and microphone usage (via PulseAudio/
// PipeWire) because browser-based calls like Google Meet may only use the mic.
func getCameraUsers() []string {
	seen := make(map[string]bool)
	var result []string

	addApp := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}

	// 1. Check PipeWire for apps capturing video.
	cmd := exec.Command("pw-dump")
	if output, err := cmd.Output(); err == nil {
		for _, app := range parsePipeWireVideoUsers(string(output)) {
			addApp(app)
		}
	}

	// 2. Scan /proc for processes with /dev/video* fds (non-PipeWire / native apps).
	entries, _ := os.ReadDir("/proc")
	for _, entry := range entries {
		pid := entry.Name()
		if !entry.IsDir() || pid[0] < '0' || pid[0] > '9' {
			continue
		}
		fdDir := filepath.Join("/proc", pid, "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			if strings.HasPrefix(target, "/dev/video") {
				comm := readProcComm(pid)
				if comm == "udevd" || comm == "systemd-udevd" || comm == "" {
					continue
				}
				addApp(mapLinuxProcessToApp(comm))
			}
		}
	}

	// 3. Check PulseAudio/PipeWire source-outputs (microphone users).
	//    A browser or known call app using the mic is a strong signal for an
	//    active video/audio call — this covers Google Meet, Zoom web, etc.
	//    even when the camera is off.
	for _, app := range getMicrophoneUsers() {
		if knownVideoCallApps[app] {
			addApp(app)
		}
	}

	return result
}

// readProcComm reads /proc/<pid>/comm and returns the trimmed process name.
func readProcComm(pid string) string {
	data, err := os.ReadFile("/proc/" + pid + "/comm")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
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

// parsePipeWireVideoUsers extracts app names from pw-dump JSON output by
// looking for nodes with media.class "Stream/Input/Video" (active video
// capture streams) and reading their application.name property.
func parsePipeWireVideoUsers(data string) []string {
	var objects []struct {
		Info struct {
			Props map[string]json.RawMessage `json:"props"`
		} `json:"info"`
	}
	if err := json.Unmarshal([]byte(data), &objects); err != nil {
		return nil
	}

	var result []string
	seen := make(map[string]bool)
	for _, obj := range objects {
		var mediaClass string
		if raw, ok := obj.Info.Props["media.class"]; ok {
			json.Unmarshal(raw, &mediaClass)
		}
		if mediaClass != "Stream/Input/Video" {
			continue
		}
		var appName string
		if raw, ok := obj.Info.Props["application.name"]; ok {
			json.Unmarshal(raw, &appName)
		}
		if appName == "" {
			continue
		}
		mapped := mapLinuxProcessToApp(appName)
		if mapped != "" && !seen[mapped] {
			seen[mapped] = true
			result = append(result, mapped)
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
