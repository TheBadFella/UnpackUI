package unpackerr

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golift.io/xtractr"
)

func TestWebServerEnabled(t *testing.T) {
	t.Parallel()

	server := &WebServer{ListenAddr: "0.0.0.0:5656"}
	if server.Enabled() {
		t.Fatal("expected disabled webserver when no feature is enabled")
	}

	server.UI = true
	if !server.Enabled() {
		t.Fatal("expected webserver to enable when UI is enabled")
	}

	server.UI = false
	server.Metrics = true
	if !server.Enabled() {
		t.Fatal("expected webserver to enable when metrics are enabled")
	}
}

func TestBuildWebStateIncludesProgress(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)
	u := New()
	u.Map["Example.Release"] = &Extract{
		App:     FolderString,
		Path:    "/downloads/Example.Release",
		Status:  EXTRACTING,
		Updated: now.Add(-30 * time.Second),
		IDs:     map[string]any{"reason": "download complete"},
		XProg: &ExtractProgress{
			Archives:  3,
			Extracted: 1,
			Progress: &xtractr.Progress{
				Total:      200,
				Wrote:      50,
				Compressed: 200,
				Read:       50,
				XFile: &xtractr.XFile{
					FilePath: "/downloads/Example.Release/file.part01.rar",
				},
			},
		},
	}

	snapshot := u.buildWebState(now)
	if snapshot.Stats.Extracting != 1 {
		t.Fatalf("expected 1 extracting item, got %d", snapshot.Stats.Extracting)
	}

	if len(snapshot.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(snapshot.Items))
	}

	progress := snapshot.Items[0].Progress
	if progress == nil {
		t.Fatal("expected progress to be present")
	}

	if progress.ArchiveIndex != 2 || progress.ArchiveCount != 3 {
		t.Fatalf("unexpected archive progress: %+v", progress)
	}

	if progress.Percent != 25 {
		t.Fatalf("expected 25 percent, got %.0f", progress.Percent)
	}
}

func TestWebStatusAPI(t *testing.T) {
	t.Parallel()

	u := New()
	u.refreshWebState(time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC))

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()

	u.webStatusAPI(rec, req, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var snapshot webStatusSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("failed to decode status payload: %v", err)
	}

	if snapshot.Stats == nil {
		t.Fatal("expected stats in payload")
	}
}
