//go:build darwin

package videocall

/*
#cgo LDFLAGS: -framework CoreMediaIO -framework CoreFoundation -framework CoreAudio

#include <CoreMediaIO/CMIOHardware.h>
#include <CoreAudio/CoreAudio.h>
#include <CoreFoundation/CoreFoundation.h>

// Check if any camera device is currently in use
int isCameraInUse() {
    CMIOObjectPropertyAddress prop = {
        kCMIOHardwarePropertyDevices,
        kCMIOObjectPropertyScopeGlobal,
        kCMIOObjectPropertyElementMain
    };

    UInt32 dataSize = 0;
    OSStatus status = CMIOObjectGetPropertyDataSize(
        kCMIOObjectSystemObject,
        &prop,
        0, NULL,
        &dataSize
    );
    if (status != kCMIOHardwareNoError) {
        return 0;
    }

    int deviceCount = dataSize / sizeof(CMIODeviceID);
    if (deviceCount == 0) {
        return 0;
    }

    CMIODeviceID *devices = (CMIODeviceID *)malloc(dataSize);
    if (devices == NULL) {
        return 0;
    }

    status = CMIOObjectGetPropertyData(
        kCMIOObjectSystemObject,
        &prop,
        0, NULL,
        dataSize,
        &dataSize,
        devices
    );
    if (status != kCMIOHardwareNoError) {
        free(devices);
        return 0;
    }

    int inUse = 0;
    CMIOObjectPropertyAddress runningProp = {
        kCMIODevicePropertyDeviceIsRunningSomewhere,
        kCMIOObjectPropertyScopeGlobal,
        kCMIOObjectPropertyElementMain
    };

    for (int i = 0; i < deviceCount; i++) {
        UInt32 isRunning = 0;
        UInt32 propSize = sizeof(isRunning);

        status = CMIOObjectGetPropertyData(
            devices[i],
            &runningProp,
            0, NULL,
            propSize,
            &propSize,
            &isRunning
        );
        if (status == kCMIOHardwareNoError && isRunning) {
            inUse = 1;
            break;
        }
    }

    free(devices);
    return inUse;
}

// Check if any audio input device is currently in use
int isMicrophoneInUse() {
    AudioObjectPropertyAddress prop = {
        kAudioHardwarePropertyDevices,
        kAudioObjectPropertyScopeGlobal,
        kAudioObjectPropertyElementMain
    };

    UInt32 dataSize = 0;
    OSStatus status = AudioObjectGetPropertyDataSize(
        kAudioObjectSystemObject,
        &prop,
        0, NULL,
        &dataSize
    );
    if (status != noErr) {
        return 0;
    }

    int deviceCount = dataSize / sizeof(AudioDeviceID);
    if (deviceCount == 0) {
        return 0;
    }

    AudioDeviceID *devices = (AudioDeviceID *)malloc(dataSize);
    if (devices == NULL) {
        return 0;
    }

    status = AudioObjectGetPropertyData(
        kAudioObjectSystemObject,
        &prop,
        0, NULL,
        &dataSize,
        devices
    );
    if (status != noErr) {
        free(devices);
        return 0;
    }

    int inUse = 0;

    for (int i = 0; i < deviceCount; i++) {
        // Check if this device has input streams
        AudioObjectPropertyAddress inputProp = {
            kAudioDevicePropertyStreams,
            kAudioDevicePropertyScopeInput,
            kAudioObjectPropertyElementMain
        };

        UInt32 streamSize = 0;
        status = AudioObjectGetPropertyDataSize(
            devices[i],
            &inputProp,
            0, NULL,
            &streamSize
        );
        if (status != noErr || streamSize == 0) {
            continue; // Not an input device
        }

        // Check if device is running
        AudioObjectPropertyAddress runningProp = {
            kAudioDevicePropertyDeviceIsRunningSomewhere,
            kAudioDevicePropertyScopeInput,
            kAudioObjectPropertyElementMain
        };

        UInt32 isRunning = 0;
        UInt32 propSize = sizeof(isRunning);

        status = AudioObjectGetPropertyData(
            devices[i],
            &runningProp,
            0, NULL,
            &propSize,
            &isRunning
        );
        if (status == noErr && isRunning) {
            inUse = 1;
            break;
        }
    }

    free(devices);
    return inUse;
}
*/
import "C"

import (
	"os/exec"
	"strings"
)

// isCameraActive checks if any camera device is currently in use on macOS
func isCameraActive() bool {
	return C.isCameraInUse() != 0
}

// isMicrophoneActive checks if any microphone is currently in use on macOS
func isMicrophoneActive() bool {
	return C.isMicrophoneInUse() != 0
}

// getCameraUsers returns a list of apps currently using the camera
func getCameraUsers() []string {
	// Check for browser-based calls first (most accurate)
	if browserCall := detectBrowserCall(); browserCall != "" {
		return []string{browserCall}
	}
	// Fall back to detecting native video call apps
	return detectNativeCallApps()
}

// getMicrophoneUsers returns a list of apps currently using the microphone
func getMicrophoneUsers() []string {
	// Same logic as camera - browser calls or native apps
	if browserCall := detectBrowserCall(); browserCall != "" {
		return []string{browserCall}
	}
	return detectNativeCallApps()
}

// detectBrowserCall checks browser tabs for video call URLs
func detectBrowserCall() string {
	// Check each browser for video call URLs
	// The script checks if app is running and gets URL in one call
	browsers := []struct {
		name   string
		script string
	}{
		{
			name: "Brave",
			script: `if application "Brave Browser" is running then
				tell application "Brave Browser" to get URL of active tab of front window
			end if`,
		},
		{
			name: "Chrome",
			script: `if application "Google Chrome" is running then
				tell application "Google Chrome" to get URL of active tab of front window
			end if`,
		},
		{
			name: "Arc",
			script: `if application "Arc" is running then
				tell application "Arc" to get URL of active tab of front window
			end if`,
		},
		{
			name: "Safari",
			script: `if application "Safari" is running then
				tell application "Safari" to get URL of current tab of front window
			end if`,
		},
		{
			name: "Edge",
			script: `if application "Microsoft Edge" is running then
				tell application "Microsoft Edge" to get URL of active tab of front window
			end if`,
		},
	}

	for _, browser := range browsers {
		cmd := exec.Command("osascript", "-e", browser.script)
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		url := strings.TrimSpace(string(output))
		if url != "" && isVideoCallURL(url) {
			return browser.name
		}
	}

	return ""
}

// isVideoCallURL checks if a URL is a known video call service
func isVideoCallURL(url string) bool {
	url = strings.ToLower(url)

	// Google Meet
	if strings.Contains(url, "meet.google.com/") {
		return true
	}
	// Zoom web client
	if strings.Contains(url, "zoom.us/j/") || strings.Contains(url, "zoom.us/wc/") {
		return true
	}
	// Microsoft Teams
	if strings.Contains(url, "teams.microsoft.com/") && strings.Contains(url, "meeting") {
		return true
	}
	// Webex
	if strings.Contains(url, "webex.com/meet/") || strings.Contains(url, "webex.com/join/") {
		return true
	}
	// Slack huddle (in browser)
	if strings.Contains(url, "slack.com/") && strings.Contains(url, "huddle") {
		return true
	}
	// Discord (web)
	if strings.Contains(url, "discord.com/channels/") {
		return true
	}

	return false
}

// detectNativeCallApps detects native video call apps that are in a call
func detectNativeCallApps() []string {
	var result []string

	// Check for Zoom call process (CptHost only runs during active calls)
	if isProcessRunning("CptHost") {
		result = append(result, "Zoom")
	}

	// Check for FaceTime call - avconferenced is always running, so check for active FaceTime window
	if isProcessRunning("FaceTime") {
		// FaceTime app must be running AND have a window (indicates active call)
		if hasWindowWithTitle("FaceTime", "") {
			result = append(result, "FaceTime")
		}
	}

	// Check for Teams call
	if isProcessRunning("MSTeams") || isProcessRunning("Microsoft Teams") {
		// Teams is trickier - check window title for "Meeting"
		if hasWindowWithTitle("Microsoft Teams", "Meeting") || hasWindowWithTitle("Microsoft Teams", "Call") {
			result = append(result, "Teams")
		}
	}

	// Check Slack huddle
	if isProcessRunning("Slack") {
		if hasWindowWithTitle("Slack", "Huddle") {
			result = append(result, "Slack")
		}
	}

	return result
}

// isProcessRunning checks if a process with the given name is running
func isProcessRunning(name string) bool {
	cmd := exec.Command("pgrep", "-x", name)
	return cmd.Run() == nil
}

// hasWindowWithTitle checks if an app has a window containing the given text
// If titleContains is empty, just checks if any window exists
func hasWindowWithTitle(appName, titleContains string) bool {
	var script string
	if titleContains == "" {
		// Just check if any window exists
		script = `tell application "System Events"
			tell process "` + appName + `"
				return (count of windows) > 0
			end tell
		end tell`
	} else {
		script = `tell application "System Events"
			tell process "` + appName + `"
				repeat with w in windows
					if name of w contains "` + titleContains + `" then return true
				end repeat
			end tell
			return false
		end tell`
	}

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}

// parseProcessList parses newline-separated process names and returns unique app names
func parseProcessList(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	seen := make(map[string]bool)
	result := make([]string, 0)

	for _, line := range lines {
		proc := strings.TrimSpace(line)
		if proc == "" {
			continue
		}

		// Map to friendly app name
		appName := mapDarwinProcessToApp(proc)
		if appName != "" && !seen[appName] {
			seen[appName] = true
			result = append(result, appName)
		}
	}

	return result
}

// mapDarwinProcessToApp maps macOS process names to user-friendly app names
func mapDarwinProcessToApp(proc string) string {
	// Normalize: remove path components, keep just process name
	if idx := strings.LastIndex(proc, "/"); idx != -1 {
		proc = proc[idx+1:]
	}

	// Map known processes
	switch {
	// Browsers
	case strings.Contains(proc, "Brave"):
		return "Brave"
	case strings.Contains(proc, "Google Chrome") || proc == "Chrome":
		return "Chrome"
	case strings.Contains(proc, "Firefox"):
		return "Firefox"
	case strings.Contains(proc, "Safari"):
		return "Safari"
	case strings.Contains(proc, "Arc"):
		return "Arc"
	case strings.Contains(proc, "Microsoft Edge") || strings.Contains(proc, "Edge"):
		return "Edge"

	// Video call apps
	case strings.Contains(proc, "zoom") || proc == "zoom.us" || proc == "CptHost":
		return "Zoom"
	case strings.Contains(proc, "Teams") || strings.Contains(proc, "MSTeams"):
		return "Teams"
	case strings.Contains(proc, "Slack"):
		return "Slack"
	case strings.Contains(proc, "Discord"):
		return "Discord"
	case strings.Contains(proc, "FaceTime") || proc == "avconferenced":
		return "FaceTime"
	case strings.Contains(proc, "Skype"):
		return "Skype"
	case strings.Contains(proc, "Webex") || strings.Contains(proc, "webex"):
		return "Webex"

	// System processes to ignore
	case proc == "kernel_task" || proc == "WindowServer" || proc == "coreaudiod" ||
		proc == "audiod" || proc == "lsof" || proc == "appleh13camerad" ||
		proc == "VDCAssistant" || proc == "AppleCameraAssistant":
		return ""

	default:
		// Return the process name as-is if we don't recognize it
		return proc
	}
}
