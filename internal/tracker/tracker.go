package tracker

import (
	"database/sql"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	keystrokesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "busygraph_keystrokes_total",
		Help: "The total number of keystrokes detected, partitioned by key",
	}, []string{"key"})
)

type KeyCount struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type TimePoint struct {
	Time  int64 `json:"time"` // Unix timestamp
	Count int   `json:"count"`
}

type VideoCallState struct {
	InCall           bool     `json:"in_call"`
	CameraActive     bool     `json:"camera_active"`
	MicrophoneActive bool     `json:"microphone_active"`
	CameraUsers      []string `json:"camera_users"`
	MicrophoneUsers  []string `json:"microphone_users"`
}

type Stats struct {
	Total     int            `json:"total"`
	KPM       KPMStats       `json:"kpm"`
	Typing    TypingStats    `json:"typing"`
	TopKeys   []KeyCount     `json:"top_keys"`
	History   []TimePoint    `json:"history"`  // Last 60 minutes
	Calendar  []TimePoint    `json:"calendar"` // Daily counts for the last year
	Mouse     MouseStats     `json:"mouse"`
	VideoCall VideoCallState `json:"video_call"`
}

type TypingStats struct {
	CharsPerBackspace float64 `json:"chars_per_backspace"`
	Backspaces        int     `json:"backspaces"`
}

// Tracker maintains the state of keystrokes
type Tracker struct {
	mu sync.Mutex
	db *sql.DB
}

// NewTracker creates a new Tracker instance and initializes DB
func NewTracker() *Tracker {
	// Determine data directory
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to get user home directory: %v", err)
		}
		dataDir = filepath.Join(home, ".local", "share")
	}

	appDir := filepath.Join(dataDir, "busygraph")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory %s: %v", appDir, err)
	}

	dbPath := filepath.Join(appDir, "busygraph.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS keystrokes (
			minute INTEGER,
			key_char TEXT,
			count INTEGER,
			PRIMARY KEY (minute, key_char)
		);
		CREATE TABLE IF NOT EXISTS mouse_metrics (
			minute INTEGER,
			metric_name TEXT,
			value REAL,
			PRIMARY KEY (minute, metric_name)
		);
		CREATE TABLE IF NOT EXISTS video_calls (
			minute INTEGER PRIMARY KEY,
			in_call INTEGER,
			camera_active INTEGER,
			microphone_active INTEGER,
			app TEXT
		);
	`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	t := &Tracker{
		db: db,
	}

	go t.flushLoop()
	return t
}

// Mouse buffering
var (
	mouseDist        float64
	mouseClicksLeft  int
	mouseClicksRight int
	mouseScroll      int
	lastMouseX       int16 = -1
	lastMouseY       int16 = -1
)

func (t *Tracker) TrackMouseClick(button string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if button == "left" {
		mouseClicksLeft++
	} else if button == "right" {
		mouseClicksRight++
	}
}

func (t *Tracker) TrackMouseScroll(amount int16) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if amount < 0 {
		mouseScroll += int(-amount)
	} else {
		mouseScroll += int(amount)
	}
}

func (t *Tracker) TrackMouseMove(x, y int16) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if lastMouseX != -1 {
		dx := float64(x - lastMouseX)
		dy := float64(y - lastMouseY)
		dist := math.Sqrt(dx*dx + dy*dy)
		mouseDist += dist
	}
	lastMouseX = x
	lastMouseY = y
}

func (t *Tracker) flushLoop() {
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		t.flushMouseMetrics()
	}
}

func (t *Tracker) flushMouseMetrics() {
	t.mu.Lock()
	defer t.mu.Unlock()

	bucket := time.Now().Truncate(time.Minute).Unix()

	// Flush logic
	metrics := map[string]float64{
		"clicks_left":  float64(mouseClicksLeft),
		"clicks_right": float64(mouseClicksRight),
		"scroll":       float64(mouseScroll),
		"distance":     mouseDist,
	}

	// Reset buffers
	mouseClicksLeft = 0
	mouseClicksRight = 0
	mouseScroll = 0
	mouseDist = 0

	for name, val := range metrics {
		if val > 0 {
			_, err := t.db.Exec(`
				INSERT INTO mouse_metrics (minute, metric_name, value) VALUES (?, ?, ?)
				ON CONFLICT(minute, metric_name) DO UPDATE SET value = value + ?
			`, bucket, name, val, val)
			if err != nil {
				log.Printf("Failed to flush mouse metric %s: %v", name, err)
			}
		}
	}
}

// Increment increases the keystroke counter for a specific key
func (t *Tracker) Increment(key string) {
	// Update Prometheus (in-memory, ephemeral)
	keystrokesTotal.WithLabelValues(key).Inc()

	// Persist to DB
	// Note: We use a simple UPSERT. For high throughput, batching would be better.
	t.mu.Lock()
	defer t.mu.Unlock()

	bucket := time.Now().Truncate(time.Minute).Unix()

	_, err := t.db.Exec(`
		INSERT INTO keystrokes (minute, key_char, count) VALUES (?, ?, 1)
		ON CONFLICT(minute, key_char) DO UPDATE SET count = count + 1
	`, bucket, key)
	if err != nil {
		log.Printf("Failed to persist keystroke: %v", err)
	}
}

func (t *Tracker) GetStats(timeRange string) Stats {
	t.mu.Lock()
	defer t.mu.Unlock()

	stats := Stats{
		Total:    0,
		TopKeys:  make([]KeyCount, 0),
		History:  make([]TimePoint, 0),
		Calendar: make([]TimePoint, 0),
	}

	now := time.Now()
	nowUnix := now.Unix()

	// Determine range config
	var startTime int64
	var groupBySeconds int64
	var points int

	switch timeRange {
	case "24h":
		startTime = now.Add(-24 * time.Hour).Unix()
		groupBySeconds = 60 // 1 minute
		points = 24 * 60
	case "7d":
		startTime = now.Add(-7 * 24 * time.Hour).Unix()
		groupBySeconds = 3600 // 1 hour
		points = 7 * 24
	case "30d":
		startTime = now.Add(-30 * 24 * time.Hour).Unix()
		groupBySeconds = 3600 // 1 hour
		points = 30 * 24
	case "1y":
		startTime = now.AddDate(-1, 0, 0).Unix()
		groupBySeconds = 86400 // 1 day
		points = 365
	default: // "1h"
		startTime = now.Add(-60 * time.Minute).Unix()
		groupBySeconds = 60 // 1 minute
		points = 60
	}

	// 1. Total (Dynamic)
	t.db.QueryRow(`SELECT COALESCE(SUM(count), 0) FROM keystrokes WHERE minute >= ?`, startTime).Scan(&stats.Total)

	// 2. Top Keys (Dynamic Range)
	rows, err := t.db.Query(`
		SELECT key_char, SUM(count) as total 
		FROM keystrokes 
		WHERE minute >= ? 
		GROUP BY key_char 
		ORDER BY total DESC 
		LIMIT 10
	`, startTime)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var kc KeyCount
			rows.Scan(&kc.Key, &kc.Count)
			stats.TopKeys = append(stats.TopKeys, kc)
		}
	}

	// 3. History (Dynamic Range & Aggregation)
	var query string
	if groupBySeconds == 60 {
		query = `SELECT minute, SUM(count) FROM keystrokes WHERE minute >= ? GROUP BY minute ORDER BY minute ASC`
	} else {
		// Aggregate by larger bucket
		// We use integer division to bucket
		query = `
			SELECT CAST(minute / ? AS INTEGER) * ? as bucket, SUM(count) 
			FROM keystrokes 
			WHERE minute >= ? 
			GROUP BY bucket
			ORDER BY bucket ASC
		`
	}

	var rowsHist *sql.Rows
	if groupBySeconds == 60 {
		rowsHist, err = t.db.Query(query, startTime)
	} else {
		rowsHist, err = t.db.Query(query, groupBySeconds, groupBySeconds, startTime)
	}

	historyMap := make(map[int64]int)
	if err == nil {
		defer rowsHist.Close()
		for rowsHist.Next() {
			var ts int64
			var cnt int
			rowsHist.Scan(&ts, &cnt)
			historyMap[ts] = cnt
		}
	}

	// Fill gaps
	// Align now to the bucket
	nowBucket := (nowUnix / groupBySeconds) * groupBySeconds
	for i := 0; i < points; i++ {
		ts := nowBucket - int64(i)*groupBySeconds
		if ts < startTime {
			break
		}
		stats.History = append(stats.History, TimePoint{
			Time:  ts,
			Count: historyMap[ts],
		})
	}
	// Reverse
	for i, j := 0, len(stats.History)-1; i < j; i, j = i+1, j-1 {
		stats.History[i], stats.History[j] = stats.History[j], stats.History[i]
	}

	// 4. Calendar (Fixed to 365 days)
	calendarStart := now.AddDate(0, 0, -365).Unix()
	rowsCal, err := t.db.Query(`
		SELECT strftime('%Y-%m-%d', minute, 'unixepoch', 'localtime') as day, SUM(count)
		FROM keystrokes
		WHERE minute >= ?
		GROUP BY day
		ORDER BY day ASC
	`, calendarStart)

	if err == nil {
		defer rowsCal.Close()
		for rowsCal.Next() {
			var dayStr string
			var cnt int
			rowsCal.Scan(&dayStr, &cnt)

			// Parse the local date string to get a timestamp for Local Midnight
			tLocal, err := time.ParseInLocation("2006-01-02", dayStr, time.Local)
			if err == nil {
				stats.Calendar = append(stats.Calendar, TimePoint{
					Time:  tLocal.Unix(),
					Count: cnt,
				})
			}
		}
	}

	// 5. Mouse Stats
	// Need to aggregate by metric name over the selected time range
	rowsMouse, err := t.db.Query(`
		SELECT metric_name, SUM(value)
		FROM mouse_metrics
		WHERE minute >= ?
		GROUP BY metric_name
	`, startTime)

	if err == nil {
		defer rowsMouse.Close()
		for rowsMouse.Next() {
			var name string
			var val float64
			rowsMouse.Scan(&name, &val)
			switch name {
			case "clicks_left":
				stats.Mouse.ClicksLeft = int(val)
			case "clicks_right":
				stats.Mouse.ClicksRight = int(val)
			case "scroll":
				stats.Mouse.Scroll = int(val)
			case "distance":
				stats.Mouse.Distance = val // Pixels
			}
		}
	}

	// 6. KPM Stats
	// Avg: Total / Minutes in range (simplified)
	minutes := float64(nowUnix-startTime) / 60.0
	if minutes < 1 {
		minutes = 1
	}
	stats.KPM.Avg = float64(stats.Total) / minutes

	// Max: The highest single minute sum in the range
	err = t.db.QueryRow(`
		SELECT COALESCE(MAX(minute_total), 0) FROM (
			SELECT SUM(count) as minute_total
			FROM keystrokes
			WHERE minute >= ?
			GROUP BY minute
		)
	`, startTime).Scan(&stats.KPM.Max)
	if err != nil {
		stats.KPM.Max = 0
	}

	// 7. Typing Stats (Characters per Backspace)
	var backspaceCount int
	err = t.db.QueryRow(`
		SELECT COALESCE(SUM(count), 0)
		FROM keystrokes
		WHERE minute >= ? AND key_char = '[BACKSPACE]'
	`, startTime).Scan(&backspaceCount)
	if err != nil {
		backspaceCount = 0
	}
	stats.Typing.Backspaces = backspaceCount

	// Calculate chars per backspace (non-backspace chars / backspaces)
	nonBackspaceChars := stats.Total - backspaceCount
	if backspaceCount > 0 {
		stats.Typing.CharsPerBackspace = float64(nonBackspaceChars) / float64(backspaceCount)
	} else {
		stats.Typing.CharsPerBackspace = 0 // No backspaces yet
	}

	return stats
}

type HeatmapPoint struct {
	Timestamp int64   `json:"ts"`
	Value     float64 `json:"value"`
}

func (t *Tracker) GetHeatmap() []HeatmapPoint {
	t.mu.Lock()
	defer t.mu.Unlock()

	data := make(map[int64]float64)

	// Keystroke counts per minute bucket
	rows, err := t.db.Query(`SELECT minute, SUM(count) FROM keystrokes GROUP BY minute`)
	if err != nil {
		log.Printf("Failed to query heatmap keystrokes: %v", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var ts int64
		var val float64
		rows.Scan(&ts, &val)
		data[ts] += val
	}

	// Mouse distance per minute bucket
	rowsMouse, err := t.db.Query(`SELECT minute, SUM(value) FROM mouse_metrics WHERE metric_name = 'distance' GROUP BY minute`)
	if err == nil {
		defer rowsMouse.Close()
		for rowsMouse.Next() {
			var ts int64
			var val float64
			rowsMouse.Scan(&ts, &val)
			data[ts] += val / 100.0
		}
	}

	result := make([]HeatmapPoint, 0, len(data))
	for ts, v := range data {
		result = append(result, HeatmapPoint{Timestamp: ts, Value: v})
	}
	return result
}

type MouseStats struct {
	Distance    float64 `json:"distance"`
	ClicksLeft  int     `json:"clicks_left"`
	ClicksRight int     `json:"clicks_right"`
	Scroll      int     `json:"scroll"`
}

type KPMStats struct {
	Avg float64 `json:"avg"`
	Max int     `json:"max"`
}

// TrackVideoCall records the current video call state
func (t *Tracker) TrackVideoCall(inCall, cameraActive, micActive bool, app string) {
	if !inCall {
		return // Only track when in a call
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	bucket := time.Now().Truncate(time.Minute).Unix()

	_, err := t.db.Exec(`
		INSERT INTO video_calls (minute, in_call, camera_active, microphone_active, app)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(minute) DO UPDATE SET
			in_call = ?,
			camera_active = MAX(camera_active, ?),
			microphone_active = MAX(microphone_active, ?),
			app = COALESCE(NULLIF(?, ''), app)
	`, bucket,
		boolToInt(inCall), boolToInt(cameraActive), boolToInt(micActive), app,
		boolToInt(inCall), boolToInt(cameraActive), boolToInt(micActive), app)
	if err != nil {
		log.Printf("Failed to track video call: %v", err)
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// VideoCallStats contains aggregated video call statistics
type VideoCallStats struct {
	TotalMinutes     int            `json:"total_minutes"`      // Total minutes in calls
	TotalCalls       int            `json:"total_calls"`        // Estimated number of calls
	CameraMinutes    int            `json:"camera_minutes"`     // Minutes with camera on
	MicrophoneMinutes int           `json:"microphone_minutes"` // Minutes with mic on
	AppBreakdown     []AppCallStats `json:"app_breakdown"`      // Per-app breakdown
	DailyMinutes     []TimePoint    `json:"daily_minutes"`      // Minutes per day
	Heatmap          []HeatmapPoint `json:"heatmap"`            // Call activity heatmap
}

type AppCallStats struct {
	App     string `json:"app"`
	Minutes int    `json:"minutes"`
}

// GetVideoCallStats returns video call statistics for the given time range
func (t *Tracker) GetVideoCallStats(timeRange string) VideoCallStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	stats := VideoCallStats{
		AppBreakdown: make([]AppCallStats, 0),
		DailyMinutes: make([]TimePoint, 0),
		Heatmap:      make([]HeatmapPoint, 0),
	}

	now := time.Now()
	var startTime int64

	switch timeRange {
	case "24h":
		startTime = now.Add(-24 * time.Hour).Unix()
	case "7d":
		startTime = now.Add(-7 * 24 * time.Hour).Unix()
	case "30d":
		startTime = now.Add(-30 * 24 * time.Hour).Unix()
	case "1y":
		startTime = now.AddDate(-1, 0, 0).Unix()
	default: // "1h"
		startTime = now.Add(-60 * time.Minute).Unix()
	}

	// Total minutes in calls
	t.db.QueryRow(`
		SELECT COALESCE(COUNT(*), 0) FROM video_calls WHERE minute >= ? AND in_call = 1
	`, startTime).Scan(&stats.TotalMinutes)

	// Camera and microphone minutes
	t.db.QueryRow(`
		SELECT COALESCE(COUNT(*), 0) FROM video_calls WHERE minute >= ? AND camera_active = 1
	`, startTime).Scan(&stats.CameraMinutes)

	t.db.QueryRow(`
		SELECT COALESCE(COUNT(*), 0) FROM video_calls WHERE minute >= ? AND microphone_active = 1
	`, startTime).Scan(&stats.MicrophoneMinutes)

	// Estimate number of calls (count gaps > 5 minutes as separate calls)
	stats.TotalCalls = t.estimateCallCount(startTime)

	// Per-app breakdown
	rows, err := t.db.Query(`
		SELECT app, COUNT(*) as minutes
		FROM video_calls
		WHERE minute >= ? AND in_call = 1 AND app != ''
		GROUP BY app
		ORDER BY minutes DESC
	`, startTime)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var appStats AppCallStats
			rows.Scan(&appStats.App, &appStats.Minutes)
			stats.AppBreakdown = append(stats.AppBreakdown, appStats)
		}
	}

	// Daily minutes (for calendar view)
	rowsDaily, err := t.db.Query(`
		SELECT strftime('%Y-%m-%d', minute, 'unixepoch', 'localtime') as day, COUNT(*) as minutes
		FROM video_calls
		WHERE minute >= ? AND in_call = 1
		GROUP BY day
		ORDER BY day ASC
	`, startTime)
	if err == nil {
		defer rowsDaily.Close()
		for rowsDaily.Next() {
			var dayStr string
			var minutes int
			rowsDaily.Scan(&dayStr, &minutes)

			tLocal, err := time.ParseInLocation("2006-01-02", dayStr, time.Local)
			if err == nil {
				stats.DailyMinutes = append(stats.DailyMinutes, TimePoint{
					Time:  tLocal.Unix(),
					Count: minutes,
				})
			}
		}
	}

	// Heatmap (minute-level data)
	rowsHeat, err := t.db.Query(`
		SELECT minute, in_call FROM video_calls WHERE minute >= ?
	`, startTime)
	if err == nil {
		defer rowsHeat.Close()
		for rowsHeat.Next() {
			var ts int64
			var inCall int
			rowsHeat.Scan(&ts, &inCall)
			stats.Heatmap = append(stats.Heatmap, HeatmapPoint{
				Timestamp: ts,
				Value:     float64(inCall),
			})
		}
	}

	return stats
}

// estimateCallCount estimates the number of separate calls by looking for gaps
func (t *Tracker) estimateCallCount(startTime int64) int {
	rows, err := t.db.Query(`
		SELECT minute FROM video_calls
		WHERE minute >= ? AND in_call = 1
		ORDER BY minute ASC
	`, startTime)
	if err != nil {
		return 0
	}
	defer rows.Close()

	var minutes []int64
	for rows.Next() {
		var m int64
		rows.Scan(&m)
		minutes = append(minutes, m)
	}

	if len(minutes) == 0 {
		return 0
	}

	// Count calls - a gap of more than 5 minutes means a new call
	calls := 1
	for i := 1; i < len(minutes); i++ {
		if minutes[i]-minutes[i-1] > 5*60 { // 5 minute gap
			calls++
		}
	}

	return calls
}

// GetVideoCallHeatmap returns heatmap data for video calls
func (t *Tracker) GetVideoCallHeatmap() []HeatmapPoint {
	t.mu.Lock()
	defer t.mu.Unlock()

	var result []HeatmapPoint

	rows, err := t.db.Query(`SELECT minute, in_call FROM video_calls WHERE in_call = 1`)
	if err != nil {
		log.Printf("Failed to query video call heatmap: %v", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var ts int64
		var inCall int
		rows.Scan(&ts, &inCall)
		result = append(result, HeatmapPoint{
			Timestamp: ts,
			Value:     float64(inCall),
		})
	}

	return result
}
