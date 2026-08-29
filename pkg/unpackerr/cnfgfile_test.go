package unpackerr

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"

	"golift.io/starr"
)

func TestValidateAppSuppressesOnlyMissingURLWarning(t *testing.T) {
	t.Parallel()

	if !New().SuppressMissingURLs {
		t.Fatal("missing URL warnings should be suppressed by default")
	}

	for _, test := range []struct {
		name     string
		suppress bool
		wantLog  bool
	}{
		{name: "suppressed by default", suppress: true, wantLog: false},
		{name: "warning enabled", suppress: false, wantLog: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer
			unpackerr := New()
			unpackerr.SuppressMissingURLs = test.suppress
			unpackerr.Error = log.New(&output, "", 0)

			err := unpackerr.validateApp(&StarrConfig{}, starr.Sonarr)
			if !errors.Is(err, ErrInvalidURL) {
				t.Fatalf("expected ErrInvalidURL, got %v", err)
			}

			logged := strings.Contains(output.String(), "Missing Sonarr URL")
			if logged != test.wantLog {
				t.Fatalf("missing URL warning logged=%v, want %v: %q", logged, test.wantLog, output.String())
			}
		})
	}
}

func TestValidateAppStillWarnsForMissingAPIKey(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	unpackerr := New()
	unpackerr.SuppressMissingURLs = true
	unpackerr.Error = log.New(&output, "", 0)

	err := unpackerr.validateApp(&StarrConfig{URL: "http://sonarr:8989"}, starr.Sonarr)
	if !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("expected ErrInvalidURL, got %v", err)
	}

	if !strings.Contains(output.String(), "Missing Sonarr API Key") {
		t.Fatalf("missing API key warning was suppressed: %q", output.String())
	}
}
