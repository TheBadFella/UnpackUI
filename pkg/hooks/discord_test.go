package hooks

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"golift.io/cnfg"
)

func TestDiscordWaitAndEditURLs(t *testing.T) {
	t.Parallel()

	waitURL := DiscordWaitURL("https://discord.com/api/webhooks/1/token")
	if waitURL != "https://discord.com/api/webhooks/1/token?wait=true" {
		t.Fatalf("unexpected wait url: %s", waitURL)
	}

	waitURL = DiscordWaitURL("https://discord.com/api/webhooks/1/token?wait=false")
	if waitURL != "https://discord.com/api/webhooks/1/token?wait=true" {
		t.Fatalf("unexpected rewritten wait url: %s", waitURL)
	}

	editURL := DiscordEditURL("https://discord.com/api/webhooks/1/token?wait=true", "99")
	if editURL != "https://discord.com/api/webhooks/1/token/messages/99" {
		t.Fatalf("unexpected edit url: %s", editURL)
	}
}

func TestSupportsDiscordUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		hook *Config
		want bool
	}{
		{
			name: "disabled",
			hook: &Config{URL: "https://discord.com/api/webhooks/1/token", UpdateExisting: false},
			want: false,
		},
		{
			name: "discord url",
			hook: &Config{URL: "https://discord.com/api/webhooks/1/token", UpdateExisting: true},
			want: true,
		},
		{
			name: "forced discord template",
			hook: &Config{URL: "https://example.com/hook", TempName: "discord", UpdateExisting: true},
			want: true,
		},
		{
			name: "notifiarr ignored",
			hook: &Config{
				URL:            "https://notifiarr.com/api/v1/notification/unpackerr/key",
				UpdateExisting: true,
			},
			want: false,
		},
		{
			name: "custom template ignored",
			hook: &Config{
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

			if got := testCase.hook.SupportsDiscordUpdate(); got != testCase.want {
				t.Fatalf("SupportsDiscordUpdate() = %v, want %v", got, testCase.want)
			}
		})
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

	hook := &Config{
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
