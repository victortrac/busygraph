package server

import (
	"embed"
	"encoding/json"
	"net/http"

	"github.com/victortrac/busygraph/internal/tracker"
)

//go:embed assets/*.html
var assets embed.FS

func RegisterDashboard(mux *http.ServeMux, t *tracker.Tracker) {
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

	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		timeRange := r.URL.Query().Get("range")
		if timeRange == "" {
			timeRange = "1h"
		}
		stats := t.GetStats(timeRange)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})
}
