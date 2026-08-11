package unpackerr

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"golift.io/cnfg"
)

func TestCleanReleaseTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "dotted release",
			in:   "Vadhandhi.S02.4K-2160p.SDR.AMZN.WEB-DL.Hindi-Tamil-Telugu.DDP5.1.HEVC.x265-HDHub4u.Ms.zip",
			want: "Vadhandhi",
		},
		{
			name: "muthassi",
			in:   "Muthassi.S01.4K-2160p.SDR.10Bit.ZEE5.WEB-DL.Hindi.DDP5.1-Malayalam.DDP5.1.HEVC.x265-HDHub4u.Ms.zip",
			want: "Muthassi",
		},
		{
			name: "path basename",
			in:   "/downloads/Muthassi.S01.2160p.WEB-DL.zip",
			want: "Muthassi",
		},
		{
			name: "starr style title",
			in:   "Some Show - S01E02 - Episode Name",
			want: "Some Show - S01E02 - Episode Name",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := discordDisplayTitle(map[string]any{"title": testCase.in}, "")
			if got != testCase.want {
				t.Fatalf("display title = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestDiscordTemplateCompactLayout(t *testing.T) {
	t.Parallel()

	hook := &WebhookConfig{
		TempName: "discord",
		Nickname: "Unpackerr",
		CType:    "application/json",
	}

	tmpl, err := hook.Template()
	if err != nil {
		t.Fatalf("template: %v", err)
	}

	payload := &WebhookPayload{
		Path:     "/downloads/Vadhandhi.S02.4K-2160p.SDR.AMZN.WEB-DL.Hindi-Tamil-Telugu.DDP5.1.HEVC.x265-HDHub4u.Ms.zip",
		App:      FolderString,
		Event:    DELETED,
		Title:    friendlyEventTitle(DELETED),
		WebURL:   "https://unpackerr.example.com",
		IDs:      map[string]any{"title": "/downloads/Vadhandhi.S02.4K-2160p.SDR.AMZN.WEB-DL.Hindi-Tamil-Telugu.DDP5.1.HEVC.x265-HDHub4u.Ms.zip"},
		Time:     time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC),
		Version:  "1.3.0",
		Revision: "1067",
		OS:       "linux",
		Arch:     "amd64",
		Data: &XtractPayload{
			Output:   "/downloads/Vadhandhi.S02.4K-2160p.SDR.AMZN.WEB-DL.Hindi-Tamil-Telugu.DDP5.1.HEVC.x265-HDHub4u.Ms",
			Archives: []string{"/downloads/file.zip"},
			Files:    []string{"/downloads/a.mkv", "/downloads/b.mkv"},
			Bytes:    45354845306,
			Elapsed:  cnfg.Duration{Duration: 50*time.Minute + 26*time.Second + 375*time.Millisecond},
			Start:    time.Date(2026, 4, 12, 11, 10, 0, 0, time.UTC),
		},
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, payload); err != nil {
		t.Fatalf("execute template: %v", err)
	}

	out := body.String()
	for _, want := range []string{
		`"title": "Vadhandhi"`,
		`"name": "Unpackerr: Source Deleted"`,
		`"name": "App"`,
		`"name": "Size"`,
		`"name": "Files"`,
		`"name": "Archives"`,
		`"name": "Elapsed"`,
		`"50m26s"`,
		`"name": "Release"`,
		`Vadhandhi.S02.4K-2160p`,
		`"name": "Links"`,
		`[Open UI](https://unpackerr.example.com)`,
		`"url": "https://unpackerr.example.com"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected template output to contain %q\n%s", want, out)
		}
	}

	for _, ban := range []string{
		`"name": "Path"`,
		`"name": "Output"`,
		`"name": "Status"`,
		`"components"`,
	} {
		if strings.Contains(out, ban) {
			t.Fatalf("did not expect template output to contain %q\n%s", ban, out)
		}
	}

	if !json.Valid(body.Bytes()) {
		t.Fatalf("discord template produced invalid JSON:\n%s", out)
	}
}
