package unpackerr

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golift.io/cnfg"
)

var (
	archiveExtSuffixes = []string{
		".tar.gz", ".tar.bz2", ".tar.xz", ".tgz",
		".zip", ".rar", ".7z", ".gz", ".bz2", ".xz", ".iso",
	}
	// Matches common scene/P2P release tags that end the show/movie title.
	mediaTokenPattern = regexp.MustCompile(`(?i)^(?:` +
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
)

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

	for _, ext := range archiveExtSuffixes {
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

	for _, sub := range strings.Split(part, "-") {
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

	return value.Local().Format("1/2/2006 3:04 PM")
}
