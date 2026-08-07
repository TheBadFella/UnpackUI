package unpackerr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	"golift.io/cnfg"
	"golift.io/xtractr"
)

type webStatusAPITestResponse struct {
	CompletedCount int                        `json:"completedCount"`
	Items          []webStatusAPITestItem     `json:"items"`
	Stats          map[string]json.RawMessage `json:"stats"`
}

type webStatusAPITestItem struct {
	Completed bool `json:"completed"`
}

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
	server.API = true
	if !server.Enabled() {
		t.Fatal("expected webserver to enable when API is enabled")
	}

	server.API = false

	server.Metrics = true
	if !server.Enabled() {
		t.Fatal("expected webserver to enable when metrics are enabled")
	}
}

func TestWebStatsAPI(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)
	unpackerr := New()
	unpackerr.Map["waiting"] = &Extract{Status: WAITING, Updated: now}
	unpackerr.Map["extracting"] = &Extract{Status: EXTRACTING, Updated: now}
	unpackerr.Map["failed"] = &Extract{Status: EXTRACTFAILED, Updated: now}
	unpackerr.Map["extracted"] = &Extract{Status: EXTRACTED, Updated: now}
	unpackerr.Map["deleted"] = &Extract{Status: DELETED, Updated: now}
	unpackerr.Finished = 12
	unpackerr.Retries = 2
	unpackerr.refreshWebState(now)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()
	unpackerr.webStatsAPI(rec, req, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var stats webStatsSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("failed to decode stats payload: %v", err)
	}

	if stats.Waiting != 1 || stats.Extracting != 1 || stats.Failed != 1 ||
		stats.Extracted != 1 || stats.Deleted != 1 {
		t.Fatalf("unexpected status counts: %+v", stats)
	}

	if stats.Finished != 12 || stats.Retries != 2 {
		t.Fatalf("unexpected counters: %+v", stats)
	}
}

func TestWebStatsRouteEnabledWithoutUI(t *testing.T) {
	t.Parallel()

	unpackerr := New()
	unpackerr.Webserver.API = true
	unpackerr.Webserver.URLBase = "/"
	unpackerr.Webserver.router = httprouter.New()
	unpackerr.webRoutes()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()
	unpackerr.Webserver.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected API-only route to return 200, got %d", rec.Code)
	}
}

func TestBuildWebStateIncludesProgress(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)
	unpackerr := New()
	unpackerr.Map["Example.Release"] = &Extract{
		App:     FolderString,
		Path:    "/downloads/Example.Release",
		Status:  EXTRACTING,
		Updated: now.Add(-30 * time.Second),
		IDs:     map[string]any{"reason": "download complete"},
		XProg: &ExtractProgress{
			Archives:  3,
			Extracted: 1,
			StartedAt: now.Add(-10 * time.Second),
			UpdatedAt: now,
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

	snapshot := unpackerr.buildWebState(now)
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

	if progress.Speed == "" {
		t.Fatal("expected extraction speed to be present")
	}

	if progress.ETA != "30s" {
		t.Fatalf("expected 30s ETA, got %q", progress.ETA)
	}
}

func TestBuildWebStateUsesFriendlyFolderDisplayName(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)
	unpackerr := New()
	unpackerr.folders = &Folders{
		Folders: map[string]*Folder{
			"/downloads/Example.Release": {
				updated: now,
				status:  EXTRACTED,
				config: &FolderConfig{
					DeleteAfter: &cnfg.Duration{Duration: 5 * time.Minute},
				},
			},
		},
	}
	unpackerr.Map["/downloads/Example.Release"] = &Extract{
		App:     FolderString,
		Path:    "/downloads/Example.Release",
		Status:  EXTRACTED,
		Updated: now,
		IDs:     map[string]any{"title": "/downloads/Example.Release"},
	}

	snapshot := unpackerr.buildWebState(now)
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(snapshot.Items))
	}

	if snapshot.Items[0].Name != "Example.Release" {
		t.Fatalf("expected display name to use folder basename, got %q", snapshot.Items[0].Name)
	}

	if snapshot.Items[0].Path != "/downloads/Example.Release" {
		t.Fatalf("expected full path to remain unchanged, got %q", snapshot.Items[0].Path)
	}

	if snapshot.Items[0].StatusText != "Extracted" {
		t.Fatalf("expected folder extract status text to be simplified, got %q", snapshot.Items[0].StatusText)
	}

	if snapshot.Items[0].DeleteIn != "5m0s" {
		t.Fatalf("expected folder delete countdown, got %q", snapshot.Items[0].DeleteIn)
	}
}

func TestBuildWebStatusDetailsFiltersMarkerFiles(t *testing.T) {
	t.Parallel()

	item := &Extract{
		App:  FolderString,
		Path: "/downloads/Example.Release",
		IDs:  map[string]any{"title": "/downloads/Example.Release"},
		Resp: &xtractr.Response{
			NewFiles: []string{
				"/downloads/Example.Release/episode.mkv",
				"/downloads/Example.Release/_unpackerred.Example.Release.txt",
				"/downloads/Example.Release/subtitles.srt",
			},
		},
	}

	details := buildWebStatusDetails(item)
	if details == nil {
		t.Fatal("expected details to be present")
	}

	if details.Title != "Example.Release" {
		t.Fatalf("expected details title to use basename, got %q", details.Title)
	}

	if len(details.Files) != 2 {
		t.Fatalf("expected marker file to be filtered out, got %d files", len(details.Files))
	}
}

func TestBuildWebStateIncludesDeleteCountdownForImportedItems(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)
	unpackerr := New()
	unpackerr.Map["Example.Release"] = &Extract{
		App:         "Sonarr",
		Path:        "/downloads/Example.Release",
		Status:      IMPORTED,
		Updated:     now,
		DeleteDelay: 10 * time.Minute,
		IDs:         map[string]any{"title": "Example Release"},
	}

	snapshot := unpackerr.buildWebState(now.Add(90 * time.Second))
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(snapshot.Items))
	}

	if snapshot.Items[0].DeleteIn != "8m30s" {
		t.Fatalf("expected imported item delete countdown, got %q", snapshot.Items[0].DeleteIn)
	}
}

func TestBuildWebStateDoesNotDuplicateCompletedItemsAcrossRefreshes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 12, 21, 53, 46, 0, time.UTC)
	unpackerr := New()
	unpackerr.folders = &Folders{
		Folders: map[string]*Folder{
			"/downloads/sample-large-zip-file.zip": {
				updated: now,
				status:  EXTRACTED,
				config: &FolderConfig{
					DeleteAfter: &cnfg.Duration{Duration: 5 * time.Minute},
				},
			},
		},
	}
	unpackerr.Map["/downloads/sample-large-zip-file.zip"] = &Extract{
		App:     FolderString,
		Path:    "/downloads/sample-large-zip-file.zip",
		Status:  EXTRACTED,
		Updated: now,
		IDs:     map[string]any{"title": "/downloads/sample-large-zip-file.zip"},
	}

	unpackerr.refreshWebState(now.Add(5 * time.Second))
	snapshot := unpackerr.buildWebState(now.Add(10 * time.Second))

	if len(snapshot.Items) != 1 {
		t.Fatalf("expected exactly one completed item after repeated refreshes, got %d", len(snapshot.Items))
	}

	if snapshot.CompletedCount != 1 {
		t.Fatalf("expected one completed item, got %d", snapshot.CompletedCount)
	}
}

func TestBuildWebStateReplacesCompletedItemWhenStatusChanges(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 12, 21, 53, 46, 0, time.UTC)
	unpackerr := New()
	path := "/downloads/sample-large-zip-file.zip"
	unpackerr.Map[path] = &Extract{
		App:     FolderString,
		Path:    path,
		Status:  EXTRACTED,
		Updated: now,
		IDs:     map[string]any{"title": path},
	}

	unpackerr.refreshWebState(now)
	unpackerr.Map[path].Status = DELETED
	unpackerr.Map[path].Updated = now.Add(time.Minute)

	snapshot := unpackerr.buildWebState(now.Add(65 * time.Second))
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected one logical item after status change, got %d", len(snapshot.Items))
	}

	if snapshot.Items[0].Status != DELETED.String() {
		t.Fatalf("expected retained item status to update to deleted, got %q", snapshot.Items[0].Status)
	}
}

func TestWebStatusAPI(t *testing.T) {
	t.Parallel()

	unpackerr := New()
	unpackerr.refreshWebState(time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()

	unpackerr.webStatusAPI(rec, req, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var snapshot webStatusAPITestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("failed to decode status payload: %v", err)
	}

	if snapshot.Stats == nil {
		t.Fatal("expected stats in payload")
	}
}

func TestBuildWebStateKeepsCompletedItemsAfterRemoval(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)
	unpackerr := New()
	unpackerr.Map["Example.Release"] = &Extract{
		App:     FolderString,
		Path:    "/downloads/Example.Release",
		Status:  DELETED,
		Updated: now,
		IDs:     map[string]any{"title": "Example Release"},
	}

	unpackerr.refreshWebState(now)
	delete(unpackerr.Map, "Example.Release")

	snapshot := unpackerr.buildWebState(now.Add(45 * time.Second))
	if len(snapshot.Items) != 1 {
		t.Fatalf("expected completed item to remain in snapshot, got %d items", len(snapshot.Items))
	}

	if !snapshot.Items[0].Completed {
		t.Fatal("expected persisted item to be marked completed")
	}

	if snapshot.CompletedCount != 1 || snapshot.ActiveCount != 0 {
		t.Fatalf("unexpected counts: active=%d completed=%d", snapshot.ActiveCount, snapshot.CompletedCount)
	}
}

func TestWebClearCompletedAPI(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)
	unpackerr := New()
	unpackerr.Map["Example.Release"] = &Extract{
		App:     FolderString,
		Path:    "/downloads/Example.Release",
		Status:  DELETED,
		Updated: now,
		IDs:     map[string]any{"title": "Example Release"},
	}
	unpackerr.refreshWebState(now)
	delete(unpackerr.Map, "Example.Release")
	unpackerr.refreshWebState(now.Add(time.Minute))

	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodPost, "/api/status/clear-completed", nil,
	)
	rec := httptest.NewRecorder()
	unpackerr.webClearCompletedAPI(rec, req, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var snapshot webStatusAPITestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("failed to decode clear payload: %v", err)
	}

	if snapshot.CompletedCount != 0 {
		t.Fatalf("expected completed items to be cleared, got %d", snapshot.CompletedCount)
	}

	if len(snapshot.Items) != 0 {
		t.Fatalf("expected no items after clear, got %d", len(snapshot.Items))
	}
}
