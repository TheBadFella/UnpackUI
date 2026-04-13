package unpackerr

import (
	"testing"
	"time"

	"golift.io/xtractr"
)

func TestBuildWebhookPayloadUsesFriendlyTitles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status ExtractStatus
		want   string
	}{
		{name: "queued", status: QUEUED, want: "New Archive Detected"},
		{name: "extracting", status: EXTRACTING, want: "Extraction Started"},
		{name: "extracted", status: EXTRACTED, want: "Extraction Complete"},
		{name: "deleted", status: DELETED, want: "Source Deleted"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			payload := buildWebhookPayload(&Extract{
				App:     FolderString,
				Path:    "/downloads/test-item",
				Status:  testCase.status,
				Updated: time.Date(2026, 4, 12, 21, 33, 0, 0, time.UTC),
				IDs:     map[string]any{"title": "test-item"},
			})

			if payload.Title != testCase.want {
				t.Fatalf("expected title %q, got %q", testCase.want, payload.Title)
			}

			if payload.Event != testCase.status {
				t.Fatalf("expected event %s, got %s", testCase.status, payload.Event)
			}
		})
	}
}

func TestBuildWebhookPayloadKeepsDataForImportedFolder(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 12, 12, 49, 0, 0, time.UTC)
	item := &Extract{
		App:     FolderString,
		Path:    "/downloads/Big.Mistakes.S01",
		Status:  IMPORTED,
		Updated: now,
		IDs:     map[string]any{"title": "Big Mistakes"},
		Resp: &xtractr.Response{
			NewFiles: []string{"/downloads/Big.Mistakes.S01/episode.mkv"},
			Output:   "/downloads/Big.Mistakes.S01_unpackerr",
			Size:     123456789,
			Queued:   1,
			Elapsed:  42 * time.Second,
			Started:  now.Add(-42 * time.Second),
			Archives: xtractr.ArchiveList{
				".rar": []string{"/downloads/Big.Mistakes.S01/archive.part01.rar"},
			},
		},
	}

	payload := buildWebhookPayload(item)
	if payload.Event != IMPORTED {
		t.Fatalf("expected imported event, got %s", payload.Event)
	}

	if payload.Data == nil {
		t.Fatal("expected extraction data to be present for imported folder payload")
	}

	if len(payload.Data.Files) != 1 || len(payload.Data.Archives) != 1 {
		t.Fatalf("expected extracted files and archives in payload, got files=%d archives=%d",
			len(payload.Data.Files), len(payload.Data.Archives))
	}
}

func TestBuildWebhookPayloadKeepsDataForDeletedEvent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 12, 12, 53, 0, 0, time.UTC)
	item := &Extract{
		App:     FolderString,
		Path:    "/downloads/Big.Mistakes.S01",
		Status:  DELETED,
		Updated: now,
		IDs:     map[string]any{"title": "Big Mistakes"},
		Resp: &xtractr.Response{
			NewFiles: []string{
				"/downloads/Big.Mistakes.S01/episode.mkv",
				"/downloads/Big.Mistakes.S01/episode.srt",
			},
			Output:  "/downloads/Big.Mistakes.S01_unpackerr",
			Size:    123456789,
			Elapsed: 42 * time.Second,
			Started: now.Add(-10 * time.Minute),
			Error:   ErrInvalidStatus,
		},
	}

	payload := buildWebhookPayload(item)
	if payload.Event != DELETED {
		t.Fatalf("expected deleted event, got %s", payload.Event)
	}

	if payload.Data == nil {
		t.Fatal("expected extraction data to be present for deleted payload")
	}

	if len(payload.Data.Files) != 2 {
		t.Fatalf("expected deleted payload to retain extracted file list, got %d files", len(payload.Data.Files))
	}

	if payload.Data.Error != ErrInvalidStatus.Error() {
		t.Fatalf("expected retained error string, got %q", payload.Data.Error)
	}
}
