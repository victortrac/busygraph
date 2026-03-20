# BusyGraph Dashboard Normalization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Normalize the BusyGraph dashboard and quick-stats window into one dual-theme visual system with denser, more consistent presentation.

**Architecture:** Keep the current embedded-HTML approach and refactor the two templates around shared design tokens, consistent section structure, and system-driven light/dark behavior. Add a lightweight server test that asserts the served pages expose the new themed shell and mini-panel structure, then update the quick-stats webview sizing to allow an adaptive layout.

**Tech Stack:** Go, `net/http`, embedded HTML/CSS/JS assets, `webview_go`, Chart.js

---

### Task 1: Add regression coverage for normalized dashboard shells

**Files:**
- Create: `internal/server/dashboard_test.go`
- Modify: `internal/server/dashboard.go`
- Test: `internal/server/dashboard_test.go`

- [ ] **Step 1: Write the failing test**

Write tests that request `/dashboard` and `/mini` from a mux registered with `RegisterDashboard`, then assert the responses contain the new shell markers such as `app-shell`, `dashboard-header`, `theme-meta`, `mini-shell`, or equivalent normalized structure.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server -run 'TestDashboard|TestMini'`
Expected: FAIL because the current templates still use the old card-grid and gradient shell.

- [ ] **Step 3: Keep handler wiring stable**

Only change server code if needed to support clean testing; keep routes and response types unchanged.

- [ ] **Step 4: Run test to confirm the failure mode is correct**

Run: `go test ./internal/server -run 'TestDashboard|TestMini'`
Expected: FAIL with missing normalized shell markers, not with a harness/setup error.

### Task 2: Normalize the main dashboard template

**Files:**
- Modify: `internal/server/assets/index.html`
- Test: `internal/server/dashboard_test.go`

- [ ] **Step 1: Replace one-off styles with theme tokens and shared panel classes**

Introduce CSS variables for surfaces, text, borders, shadows, spacing, and semantic accents. Remove the inline-style-heavy shell in favor of reusable classes.

- [ ] **Step 2: Rework the dashboard structure around the approved design**

Keep the heatmaps prominent, convert the range selector into a segmented control, left-align and tighten section headers, and group overview/keyboard/mouse/video-call blocks into one coherent system.

- [ ] **Step 3: Update charts and JS hooks to match the new shell**

Keep all existing IDs and endpoints working, but update button state handling, labels, and chart color assignments so they honor the new token system and both themes.

- [ ] **Step 4: Run focused tests**

Run: `go test ./internal/server -run TestDashboard`
Expected: PASS

### Task 3: Normalize the quick-stats window and make the window adaptive

**Files:**
- Modify: `internal/server/assets/mini.html`
- Modify: `main.go`
- Test: `internal/server/dashboard_test.go`

- [ ] **Step 1: Replace the gradient/glass widget styling**

Restyle the mini view as a compact sibling of the dashboard using the same token system, denser metric treatment, and dark/light theme behavior.

- [ ] **Step 2: Make the mini layout adaptive**

Remove the hidden-overflow/fixed-toy composition so the panel can breathe at different sizes and larger text settings.

- [ ] **Step 3: Update the webview sizing hint**

Switch the quick-stats window away from fixed sizing so the normalized layout can resize appropriately.

- [ ] **Step 4: Run focused tests**

Run: `go test ./internal/server -run TestMini`
Expected: PASS

### Task 4: Verify the full integration

**Files:**
- Modify: `internal/server/assets/index.html`
- Modify: `internal/server/assets/mini.html`
- Modify: `internal/server/dashboard_test.go`
- Modify: `main.go`

- [ ] **Step 1: Run package tests**

Run: `go test ./internal/server`
Expected: PASS

- [ ] **Step 2: Run full project verification**

Run: `go test ./...`
Expected: PASS, allowing existing systray deprecation warnings from the Linux build.

- [ ] **Step 3: Review the diff**

Run: `git diff -- internal/server/assets/index.html internal/server/assets/mini.html internal/server/dashboard_test.go main.go`
Expected: only the planned dashboard normalization changes.

- [ ] **Step 4: Commit**

```bash
git add internal/server/assets/index.html internal/server/assets/mini.html internal/server/dashboard_test.go main.go
git commit -m "Normalize BusyGraph dashboard surfaces"
```
