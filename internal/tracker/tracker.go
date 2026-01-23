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

type Stats struct {
	Total    int         `json:"total"`
	KPM      KPMStats    `json:"kpm"`
	Typing   TypingStats `json:"typing"`
	TopKeys  []KeyCount  `json:"top_keys"`
	History  []TimePoint `json:"history"`  // Last 60 minutes
	Calendar []TimePoint `json:"calendar"` // Daily counts for the last year
	Mouse    MouseStats  `json:"mouse"`
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

	// Create table
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
