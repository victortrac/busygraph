package server

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/victortrac/busygraph/internal/tracker"
)

// Start starts the metrics server on the given port
func Start(addr string, t *tracker.Tracker) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	RegisterDashboard(mux, t)

	log.Printf("Starting metrics server on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
