package unpackerr

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecoveryTracksAndClearsFolder(t *testing.T) {
	t.Parallel()

	stateFile := filepath.Join(t.TempDir(), defaultStateFile)
	watchPath := t.TempDir()
	archivePath := filepath.Join(watchPath, "movie.zip")
	now := time.Now().UTC()

	unpackerr := New()
	unpackerr.StateFile = stateFile
	unpackerr.recovery = newRecoveryState()

	cfg := &FolderConfig{Path: watchPath}
	unpackerr.recoveryTrackFolder(archivePath, cfg, EXTRACTING, now)

	if _, err := os.Stat(stateFile); err != nil {
		t.Fatalf("expected recovery state file to exist: %v", err)
	}

	state, err := readRecoveryState(stateFile)
	if err != nil {
		t.Fatalf("reading recovery state: %v", err)
	}

	item := state.Folders[archivePath]
	if item == nil {
		t.Fatalf("expected recovery item for %s", archivePath)
	}

	if item.Status != EXTRACTING.String() {
		t.Fatalf("expected status %q, got %q", EXTRACTING.String(), item.Status)
	}

	if item.WatchPath != filepath.Clean(watchPath) {
		t.Fatalf("expected watch path %q, got %q", filepath.Clean(watchPath), item.WatchPath)
	}

	unpackerr.recoveryClearFolder(archivePath)

	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Fatalf("expected empty recovery state file to be removed, got err=%v", err)
	}
}

func TestRecoverInterruptedFolders(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()

	archivePath := filepath.Join(watchPath, "movie.zip")
	if err := os.WriteFile(archivePath, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("creating archive placeholder: %v", err)
	}

	now := time.Now().UTC()
	unpackerr := New()
	unpackerr.Folders = []*FolderConfig{{Path: watchPath}}
	unpackerr.StateFile = filepath.Join(t.TempDir(), defaultStateFile)
	unpackerr.folders = &Folders{
		Folders: make(map[string]*Folder),
		Outputs: make(map[string]string),
	}
	unpackerr.recovery = &recoveryState{
		Version: recoveryStateVersion,
		Folders: map[string]*recoveryFolder{
			archivePath: {
				Path:      archivePath,
				WatchPath: watchPath,
				Status:    EXTRACTING.String(),
				Updated:   now.Add(-time.Minute),
			},
		},
	}

	unpackerr.recoverInterruptedFolders(now)

	folder := unpackerr.folders.Folders[archivePath]
	if folder == nil {
		t.Fatalf("expected interrupted folder to be restored")
	}

	if folder.status != WAITING {
		t.Fatalf("expected restored folder to be waiting, got %s", folder.status)
	}

	if now.Sub(folder.updated) < unpackerr.StartDelay.Duration {
		t.Fatalf("expected extracting item to be eligible for immediate retry")
	}
}

func TestRecoverInterruptedFolderCleansPartialOutput(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()
	extractPath := t.TempDir()
	archivePath := filepath.Join(watchPath, "movie.zip")
	tempOutput := filepath.Join(extractPath, "movie.zip"+suffix)
	finalOutput := filepath.Join(extractPath, "movie")

	if err := os.WriteFile(archivePath, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("creating archive placeholder: %v", err)
	}

	if err := os.MkdirAll(tempOutput, 0o700); err != nil {
		t.Fatalf("creating temp partial output: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tempOutput, "partial.tmp"), []byte("partial"), 0o600); err != nil {
		t.Fatalf("writing temp partial output: %v", err)
	}

	if err := os.MkdirAll(finalOutput, 0o700); err != nil {
		t.Fatalf("creating final partial output: %v", err)
	}

	if err := os.WriteFile(filepath.Join(finalOutput, "partial.tmp"), []byte("partial"), 0o600); err != nil {
		t.Fatalf("writing final partial output: %v", err)
	}

	now := time.Now().UTC()
	unpackerr := newRecoveryTestUnpackerr(watchPath, archivePath, EXTRACTING, now.Add(-time.Minute))
	unpackerr.Folders[0].ExtractPath = extractPath

	unpackerr.recoverInterruptedFolders(now)

	if _, err := os.Stat(tempOutput); !os.IsNotExist(err) {
		t.Fatalf("expected temp partial output to be cleaned, got err=%v", err)
	}

	if _, err := os.Stat(finalOutput); !os.IsNotExist(err) {
		t.Fatalf("expected final partial output to be cleaned, got err=%v", err)
	}

	if folder := unpackerr.folders.Folders[archivePath]; folder == nil || folder.status != WAITING {
		t.Fatalf("expected interrupted folder to be restored for retry, got %#v", folder)
	}
}

func TestRecoverWaitingFolderKeepsOriginalUpdatedTime(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()

	archivePath := filepath.Join(watchPath, "movie.zip")
	if err := os.WriteFile(archivePath, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("creating archive placeholder: %v", err)
	}

	updated := time.Now().UTC()
	now := updated.Add(time.Second)
	unpackerr := New()
	unpackerr.Folders = []*FolderConfig{{Path: watchPath}}
	unpackerr.StateFile = filepath.Join(t.TempDir(), defaultStateFile)
	unpackerr.folders = &Folders{
		Folders: make(map[string]*Folder),
		Outputs: make(map[string]string),
	}
	unpackerr.recovery = &recoveryState{
		Version: recoveryStateVersion,
		Folders: map[string]*recoveryFolder{
			archivePath: {
				Path:      archivePath,
				WatchPath: watchPath,
				Status:    WAITING.String(),
				Updated:   updated,
			},
		},
	}

	unpackerr.recoverInterruptedFolders(now)

	folder := unpackerr.folders.Folders[archivePath]
	if folder == nil {
		t.Fatalf("expected waiting folder to be restored")
	}

	if !folder.updated.Equal(updated) {
		t.Fatalf("expected original updated time %s, got %s", updated, folder.updated)
	}
}

func TestRecoverWaitingFolderDoesNotCleanOutput(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()
	archivePath := filepath.Join(watchPath, "movie.zip")
	finalOutput := filepath.Join(watchPath, "movie")

	if err := os.WriteFile(archivePath, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("creating archive placeholder: %v", err)
	}

	if err := os.MkdirAll(finalOutput, 0o700); err != nil {
		t.Fatalf("creating existing output: %v", err)
	}

	now := time.Now().UTC()
	unpackerr := newRecoveryTestUnpackerr(watchPath, archivePath, WAITING, now.Add(-time.Minute))

	unpackerr.recoverInterruptedFolders(now)

	if _, err := os.Stat(finalOutput); err != nil {
		t.Fatalf("expected waiting recovery to leave output alone: %v", err)
	}
}

func newRecoveryTestUnpackerr(watchPath, archivePath string, status ExtractStatus, updated time.Time) *Unpackerr {
	unpackerr := New()
	unpackerr.Folders = []*FolderConfig{{Path: watchPath}}
	unpackerr.folders = &Folders{
		Folders: make(map[string]*Folder),
		Outputs: make(map[string]string),
	}
	unpackerr.recovery = &recoveryState{
		Version: recoveryStateVersion,
		Folders: map[string]*recoveryFolder{
			archivePath: {
				Path:      archivePath,
				WatchPath: watchPath,
				Status:    status.String(),
				Updated:   updated,
			},
		},
	}

	return unpackerr
}
