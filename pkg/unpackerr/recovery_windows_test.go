package unpackerr

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRecoveryFolderConfigMatchesWindowsCaseWithoutEscapingRoot(t *testing.T) {
	t.Parallel()

	watchPath := filepath.Join(t.TempDir(), "Watch")
	archivePath := filepath.Join(watchPath, "movie.zip")
	unpackerr := New()
	unpackerr.Folders = InstanceMap[FolderConfig]{"0": {Path: watchPath}}

	if got := unpackerr.recoveryFolderConfig(strings.ToLower(archivePath), strings.ToLower(watchPath)); got == nil {
		t.Fatal("case-only Windows path change did not match the configured watch folder")
	}

	outside := strings.ToLower(watchPath) + "-other"
	if got := unpackerr.recoveryFolderConfig(filepath.Join(outside, "movie.zip"), strings.ToLower(watchPath)); got != nil {
		t.Fatalf("sibling path escaped watch root: %+v", got)
	}
}
