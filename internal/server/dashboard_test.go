package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDashboardServesNormalizedShell(t *testing.T) {
	body := fetchDashboardPage(t, "/dashboard")

	for _, marker := range []string{
		`class="app-shell"`,
		`class="dashboard-header"`,
		`class="range-control"`,
		`data-theme="system"`,
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("dashboard page missing marker %q", marker)
		}
	}
}

func TestDashboardCentersAnnualCalendar(t *testing.T) {
	body := fetchDashboardPage(t, "/dashboard")

	start := strings.Index(body, `.calendar-shell {`)
	if start == -1 {
		t.Fatal("dashboard page missing .calendar-shell rule")
	}

	block := body[start:]
	end := strings.Index(block, "}")
	if end == -1 {
		t.Fatal("dashboard page has unterminated .calendar-shell rule")
	}

	calendarRule := block[:end]
	if !strings.Contains(calendarRule, `justify-content: center;`) {
		t.Fatalf("calendar shell rule does not center its content:\n%s", calendarRule)
	}
}

func TestMiniServesNormalizedShell(t *testing.T) {
	body := fetchDashboardPage(t, "/mini")

	for _, marker := range []string{
		`class="mini-shell"`,
		`class="mini-panel"`,
		`data-theme="system"`,
		`class="mini-meta"`,
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("mini page missing marker %q", marker)
		}
	}
}

func fetchDashboardPage(t *testing.T, path string) string {
	t.Helper()

	mux := http.NewServeMux()
	RegisterDashboard(mux, nil, nil)

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code %d", rec.Code)
	}

	return rec.Body.String()
}
