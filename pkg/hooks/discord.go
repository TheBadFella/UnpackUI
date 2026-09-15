package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Unpackerr/unpackerr/pkg/extract"
	"golift.io/cnfg"
)

// Matches common scene/P2P release tags that end the show/movie title.
var mediaTokenPattern = regexp.MustCompile(`(?i)^(?:` +
	`s\d{1,2}(?:e\d{1,3})?|` + // S01 / S01E02
	`\d{1,2}x\d{1,3}|` + // 1x02
	`\d{3,4}p|4k|8k|uhd|hdr|sdr|dv|` + // 1080p / 4K / HDR
	`web-?dl|webrip|bluray|b[dr]rip|hdtv|dvdrip|remux|` +
	`x264|x265|h\.?264|h\.?265|hevc|avc|av1|xvid|` +
	`ddp?(?:\d(?:\.\d)?)?|aac|dts(?:-?hd)?|truehd|atmos|flac|pcm|` +
	`proper|repack|internal|extended|unrated|directors?\.?cut|` +
	`multi|dual|limited|complete|season|` +
	`amzn|nf|dsnp|hulu|atvp|pcok|hmax|zee5|hotstar|` +
	`\d{4}` + // year
	`)$`)

func archiveExtSuffixes() []string {
	return []string{
		".tar.gz", ".tar.bz2", ".tar.xz", ".tgz",
		".zip", ".rar", ".7z", ".gz", ".bz2", ".xz", ".iso",
	}
}

// discordReleaseName returns the raw release/file name for Discord embeds.
func discordReleaseName(ids map[string]any, path string) string {
	if title, ok := ids["title"]; ok {
		if raw := strings.TrimSpace(fmt.Sprint(title)); raw != "" {
			return filepath.Base(filepath.Clean(raw))
		}
	}

	if path != "" {
		return filepath.Base(filepath.Clean(path))
	}

	return "Unknown"
}

// discordDisplayTitle returns a short, human-friendly title for Discord embeds.
func discordDisplayTitle(ids map[string]any, path string) string {
	raw := discordReleaseName(ids, path)
	cleaned := cleanReleaseTitle(raw)

	if cleaned != "" {
		return cleaned
	}

	return raw
}

func cleanReleaseTitle(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return ""
	}

	name = stripArchiveExtension(name)

	// Starr titles are usually already readable ("Show Name - S01E02").
	if strings.Contains(name, " ") && !strings.Contains(name, ".") {
		return name
	}

	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '.' || r == '_'
	})
	if len(parts) == 0 {
		return name
	}

	titleParts := make([]string, 0, len(parts))

	for _, part := range parts {
		if isMediaToken(part) {
			break
		}

		titleParts = append(titleParts, part)
	}

	if len(titleParts) == 0 {
		return strings.ReplaceAll(strings.ReplaceAll(name, ".", " "), "_", " ")
	}

	return strings.Join(titleParts, " ")
}

func stripArchiveExtension(name string) string {
	lower := strings.ToLower(name)

	for _, ext := range archiveExtSuffixes() {
		if strings.HasSuffix(lower, ext) {
			return name[:len(name)-len(ext)]
		}
	}

	if ext := filepath.Ext(name); len(ext) > 1 && len(ext) <= 5 {
		return strings.TrimSuffix(name, ext)
	}

	return name
}

func isMediaToken(part string) bool {
	if mediaTokenPattern.MatchString(part) {
		return true
	}

	for sub := range strings.SplitSeq(part, "-") {
		if sub != "" && mediaTokenPattern.MatchString(sub) {
			return true
		}
	}

	return false
}

func shortDuration(duration cnfg.Duration) string {
	if duration.Duration <= 0 {
		return ""
	}

	return duration.Duration.Round(time.Second).String()
}

func formatDiscordTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	// Discord timestamps should match the host clock, not UTC.
	return value.In(time.Local).Format("1/2/2006 3:04 PM") //nolint:gosmopolitan
}

// SendOrUpdate POSTs a new Discord webhook message (with wait=true) or PATCHes
// an existing one when update_existing is enabled. clearKey removes the stored
// message ID after a successful update (used for terminal extract events).
func (w *Config) SendOrUpdate(key string, body []byte, clearKey bool) ([]byte, error) {
	if w.URL == "" {
		return nil, ErrWebhookNoURL
	}

	w.Lock()
	defer w.Unlock()

	if w.msgIDs == nil {
		w.msgIDs = make(map[string]string)
	}

	w.posts++

	ctx, cancel := context.WithTimeout(context.Background(), w.Timeout.Duration+time.Second)
	defer cancel()

	if msgID := w.msgIDs[key]; msgID != "" {
		reply, status, err := w.sendRequest(ctx, http.MethodPatch, DiscordEditURL(w.URL, msgID), bytes.NewReader(body))
		if err == nil {
			if clearKey {
				delete(w.msgIDs, key)
			}

			return reply, nil
		}

		// Missing or expired message — create a new one below.
		if status != http.StatusNotFound && status != http.StatusUnauthorized {
			w.fails++
			return reply, err
		}

		delete(w.msgIDs, key)
	}

	reply, _, err := w.sendRequest(ctx, http.MethodPost, DiscordWaitURL(w.URL), bytes.NewReader(body))
	if err != nil {
		w.fails++
		return reply, err
	}

	var msg struct {
		ID string `json:"id"`
	}

	if json.Unmarshal(reply, &msg) == nil && msg.ID != "" && !clearKey {
		w.msgIDs[key] = msg.ID
	}

	return reply, nil
}

func (w *Config) sendRequest(ctx context.Context, method, rawURL string, body io.Reader) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", w.CType)

	client := w.client
	if client == nil {
		client = http.DefaultClient
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%sing payload: %w", method, err)
	}
	defer res.Body.Close()

	// Read the body to avoid a memory leak; used when status is unexpected.
	reply, _ := io.ReadAll(res.Body)

	if res.StatusCode < http.StatusOK || res.StatusCode > http.StatusNoContent {
		return reply, res.StatusCode, fmt.Errorf("%w (%s): %s", ErrInvalidStatus, res.Status, reply)
	}

	return reply, res.StatusCode, nil
}

// SupportsDiscordUpdate reports whether this webhook should create-then-edit
// Discord messages instead of posting a new message per event.
func (w *Config) SupportsDiscordUpdate() bool {
	if w == nil || !w.UpdateExisting {
		return false
	}

	if strings.EqualFold(w.TempName, "discord") {
		return true
	}

	// Custom or forced non-Discord templates are fire-and-forget only.
	if w.TempName != "" || w.TmplPath != "" {
		return false
	}

	lower := strings.ToLower(w.URL)

	return strings.Contains(lower, "discord.com") || strings.Contains(lower, "discordapp.com")
}

func isTerminalWebhookEvent(event extract.Status) bool {
	switch event {
	case extract.DELETED, extract.EXTRACTEDNOTHING:
		return true
	default:
		return false
	}
}

func DiscordWaitURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		if strings.Contains(rawURL, "?") {
			return rawURL + "&wait=true"
		}

		return rawURL + "?wait=true"
	}

	query := parsed.Query()
	query.Set("wait", "true")
	parsed.RawQuery = query.Encode()

	return parsed.String()
}

func DiscordEditURL(rawURL, messageID string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		base, _, _ := strings.Cut(rawURL, "?")

		return strings.TrimRight(base, "/") + "/messages/" + messageID
	}

	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/messages/" + messageID

	return parsed.String()
}
