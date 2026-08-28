package unpackerr

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"golift.io/cnfg"
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

			payload := (&Unpackerr{Config: &Config{}}).buildWebhookPayload(&Extract{
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

	payload := (&Unpackerr{Config: &Config{}}).buildWebhookPayload(item)
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
		Retries: 2,
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

	payload := (&Unpackerr{Config: &Config{}}).buildWebhookPayload(item)
	if payload.Event != DELETED {
		t.Fatalf("expected deleted event, got %s", payload.Event)
	}

	if payload.Retries != 2 {
		t.Fatalf("expected retries 2, got %d", payload.Retries)
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

func TestDiscordWaitAndEditURLs(t *testing.T) {
	t.Parallel()

	waitURL := discordWaitURL("https://discord.com/api/webhooks/1/token")
	if waitURL != "https://discord.com/api/webhooks/1/token?wait=true" {
		t.Fatalf("unexpected wait url: %s", waitURL)
	}

	waitURL = discordWaitURL("https://discord.com/api/webhooks/1/token?wait=false")
	if waitURL != "https://discord.com/api/webhooks/1/token?wait=true" {
		t.Fatalf("unexpected rewritten wait url: %s", waitURL)
	}

	editURL := discordEditURL("https://discord.com/api/webhooks/1/token?wait=true", "99")
	if editURL != "https://discord.com/api/webhooks/1/token/messages/99" {
		t.Fatalf("unexpected edit url: %s", editURL)
	}
}

func TestSupportsDiscordUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		hook *WebhookConfig
		want bool
	}{
		{
			name: "disabled",
			hook: &WebhookConfig{URL: "https://discord.com/api/webhooks/1/token", UpdateExisting: false},
			want: false,
		},
		{
			name: "discord url",
			hook: &WebhookConfig{URL: "https://discord.com/api/webhooks/1/token", UpdateExisting: true},
			want: true,
		},
		{
			name: "forced discord template",
			hook: &WebhookConfig{URL: "https://example.com/hook", TempName: "discord", UpdateExisting: true},
			want: true,
		},
		{
			name: "notifiarr ignored",
			hook: &WebhookConfig{
				URL:            "https://notifiarr.com/api/v1/notification/unpackerr/key",
				UpdateExisting: true,
			},
			want: false,
		},
		{
			name: "custom template ignored",
			hook: &WebhookConfig{
				URL:            "https://discord.com/api/webhooks/1/token",
				TmplPath:       "/tmp/custom.tmpl",
				UpdateExisting: true,
			},
			want: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := testCase.hook.supportsDiscordUpdate(); got != testCase.want {
				t.Fatalf("supportsDiscordUpdate() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestDiscordTemplateIncludesExtraFields(t *testing.T) {
	t.Parallel()
	assertDiscordTemplateContains(t, extraFieldsDiscordTestPayload(), []string{
		`"title": "Movie Name"`,
		`"name": "Unpackerr: Extraction Complete"`,
		`"name": "App"`,
		`"name": "Retries"`,
		`"name": "Release"`,
		`"name": "Reason"`,
		`"name": "Links"`,
		`[Open UI](http://localhost:5656)`,
		`"url": "http://localhost:5656"`,
	}, nil)
}

func extraFieldsDiscordTestPayload() *WebhookPayload {
	return &WebhookPayload{
		Path:    "/downloads/Movie.Name.2024.1080p.WEB-DL.mkv",
		App:     "radarr",
		Event:   EXTRACTED,
		Title:   friendlyEventTitle(EXTRACTED),
		Retries: 1,
		WebURL:  "http://localhost:5656",
		IDs: map[string]any{
			"title":      "Movie Name",
			"downloadId": "abc-123",
			"reason":     "Download completed",
		},
		Time:     time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC),
		Version:  "1.0.0",
		Revision: "abc",
		OS:       "linux",
		Arch:     "amd64",
		Data: &XtractPayload{
			Output:   "/downloads/Movie.Name_unpackerr",
			Archives: []string{"/downloads/Movie.Name/a.rar"},
			Files:    []string{"/downloads/Movie.Name/movie.mkv"},
			Bytes:    1024,
			Elapsed:  cnfg.Duration{Duration: 5 * time.Second},
			Queue:    2,
			Start:    time.Date(2026, 4, 12, 11, 59, 55, 0, time.UTC),
		},
	}
}

func TestSendOrUpdateCreatesThenEditsDiscordMessage(t *testing.T) {
	t.Parallel()

	var (
		requestLock sync.Mutex
		requests    []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestLock.Lock()
		requests = append(requests, request.Method+" "+request.URL.RequestURI())
		requestLock.Unlock()

		switch {
		case request.Method == http.MethodPost && request.URL.Query().Get("wait") == "true":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"id":"msg-1"}`))
		case request.Method == http.MethodPatch && strings.HasSuffix(request.URL.Path, "/messages/msg-1"):
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(`{"id":"msg-1"}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	hook := &WebhookConfig{
		URL:            server.URL + "/api/webhooks/1/token",
		CType:          "application/json",
		UpdateExisting: true,
		Timeout:        cnfg.Duration{Duration: time.Second},
		client:         server.Client(),
	}

	if _, err := hook.SendOrUpdate("/downloads/item", []byte(`{"content":"one"}`), false); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := hook.SendOrUpdate("/downloads/item", []byte(`{"content":"two"}`), true); err != nil {
		t.Fatalf("update: %v", err)
	}

	requestLock.Lock()
	defer requestLock.Unlock()

	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %#v", requests)
	}

	if !strings.HasPrefix(requests[0], "POST ") || !strings.Contains(requests[0], "wait=true") {
		t.Fatalf("expected create with wait=true, got %q", requests[0])
	}

	if !strings.HasPrefix(requests[1], "PATCH ") || !strings.Contains(requests[1], "/messages/msg-1") {
		t.Fatalf("expected patch to message id, got %q", requests[1])
	}

	if len(hook.msgIDs) != 0 {
		t.Fatalf("expected message id cleared after terminal update, got %#v", hook.msgIDs)
	}
}
