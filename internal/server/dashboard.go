package server

import (
	"embed"
	"encoding/json"
	"net/http"

	"github.com/victortrac/busygraph/internal/tracker"
	"github.com/victortrac/busygraph/internal/videocall"
)

//go:embed assets/*.html
var assets embed.FS

func RegisterDashboard(mux *http.ServeMux, t *tracker.Tracker, vc videocall.Detector) {
	mux.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		content, _ := assets.ReadFile("assets/index.html")
		w.Header().Set("Content-Type", "text/html")
		w.Write(content)
	})

	mux.HandleFunc("/mini", func(w http.ResponseWriter, r *http.Request) {
		content, _ := assets.ReadFile("assets/mini.html")
		w.Header().Set("Content-Type", "text/html")
		w.Write(content)
	})

	mux.HandleFunc("/api/heatmap", func(w http.ResponseWriter, r *http.Request) {
		data := t.GetHeatmap()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	})

	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		timeRange := r.URL.Query().Get("range")
		if timeRange == "" {
			timeRange = "1h"
		}
		stats := t.GetStats(timeRange)

		// Add video call state to stats
		if vc != nil {
			vcState := vc.GetState()
			stats.VideoCall = tracker.VideoCallState{
				InCall:           vcState.InCall,
				CameraActive:     vcState.CameraActive,
				MicrophoneActive: vcState.MicrophoneActive,
				CameraUsers:      vcState.CameraUsers,
				MicrophoneUsers:  vcState.MicrophoneUsers,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})

	mux.HandleFunc("/api/videocall", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if vc != nil {
			json.NewEncoder(w).Encode(vc.GetState())
		} else {
			json.NewEncoder(w).Encode(videocall.CallState{})
		}
	})

	mux.HandleFunc("/api/videocall/stats", func(w http.ResponseWriter, r *http.Request) {
		timeRange := r.URL.Query().Get("range")
		if timeRange == "" {
			timeRange = "24h"
		}
		stats := t.GetVideoCallStats(timeRange)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})

	mux.HandleFunc("/api/videocall/heatmap", func(w http.ResponseWriter, r *http.Request) {
		data := t.GetVideoCallHeatmap()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	})
}
