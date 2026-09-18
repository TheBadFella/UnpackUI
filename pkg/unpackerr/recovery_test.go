package unpackerr

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golift.io/cnfg"
	"golift.io/xtractr"
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
	unpackerr.Folders = InstanceMap[FolderConfig]{"0": {Path: watchPath}}
	unpackerr.StateFile = filepath.Join(t.TempDir(), defaultStateFile)
	unpackerr.folders = &Folders{
		Folders: make(map[string]*Folder),
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

	if folder.Status != WAITING {
		t.Fatalf("expected restored folder to be waiting, got %s", folder.Status)
	}

	if now.Sub(folder.Updated) < unpackerr.StartDelay.Duration {
		t.Fatalf("expected extracting item to be eligible for immediate retry")
	}
}

func TestRecoveryFolderConfigPrefersNestedWatchPath(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	child := filepath.Join(parent, "tv")
	archive := filepath.Join(child, "episode.zip")

	unpackerr := New()
	unpackerr.Folders = InstanceMap[FolderConfig]{
		"parent": {Path: parent},
		"child":  {Path: child},
	}

	got := unpackerr.recoveryFolderConfig(archive, ".")
	if got == nil || filepath.Clean(got.Path) != filepath.Clean(child) {
		t.Fatalf("recovery config = %+v, want nested watch %q", got, child)
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
	unpackerr.Folders["0"].ExtractPath = extractPath

	unpackerr.recoverInterruptedFolders(now)

	if _, err := os.Stat(tempOutput); !os.IsNotExist(err) {
		t.Fatalf("expected temp partial output to be cleaned, got err=%v", err)
	}

	if _, err := os.Stat(finalOutput); !os.IsNotExist(err) {
		t.Fatalf("expected final partial output to be cleaned, got err=%v", err)
	}

	if folder := unpackerr.folders.Folders[archivePath]; folder == nil || folder.Status != WAITING {
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
	unpackerr.Folders = InstanceMap[FolderConfig]{"0": {Path: watchPath}}
	unpackerr.StateFile = filepath.Join(t.TempDir(), defaultStateFile)
	unpackerr.folders = &Folders{
		Folders: make(map[string]*Folder),
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

	if !folder.Updated.Equal(updated) {
		t.Fatalf("expected original updated time %s, got %s", updated, folder.Updated)
	}
}

//nolint:funlen // restart sequence is the assertion.
func TestRecoverExtractedFolderKeepsDeleteDeadlineAcrossRestarts(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()
	archivePath := filepath.Join(watchPath, "movie.zip")
	if err := os.WriteFile(archivePath, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("creating archive placeholder: %v", err)
	}

	stateFile := filepath.Join(t.TempDir(), defaultStateFile)
	updated := time.Now().UTC().Add(-time.Minute)
	deleteAfter := time.Hour
	cfg := &FolderConfig{Path: watchPath, DeleteAfter: &cnfg.Duration{Duration: deleteAfter}}

	unpackerr := New()
	unpackerr.Folders = InstanceMap[FolderConfig]{"0": cfg}
	unpackerr.StateFile = stateFile
	unpackerr.recovery = newRecoveryState()
	unpackerr.recoveryTrackFolder(archivePath, cfg, EXTRACTED, updated)

	firstState, err := readRecoveryState(stateFile)
	if err != nil {
		t.Fatalf("reading first recovery state: %v", err)
	}
	item := firstState.Folders[archivePath]
	if item == nil || item.Status != EXTRACTED.String() || !item.Updated.Equal(updated) {
		t.Fatalf("saved extracted recovery item = %+v", item)
	}

	unpackerr.folders = &Folders{Folders: make(map[string]*Folder)}
	firstRestart := updated.Add(2 * time.Minute)
	unpackerr.recoverInterruptedFolders(firstRestart)

	folder := unpackerr.folders.Folders[archivePath]
	if folder == nil || folder.Status != EXTRACTED {
		t.Fatalf("first restart folder = %+v", folder)
	}
	if !folder.Updated.Equal(updated) {
		t.Fatalf("first restart updated = %s, want %s", folder.Updated, updated)
	}
	if got, want := folder.Updated.Add(folder.Config.DeleteAfter.Duration), updated.Add(deleteAfter); !got.Equal(want) {
		t.Fatalf("first restart delete deadline = %s, want %s", got, want)
	}

	secondState, err := readRecoveryState(stateFile)
	if err != nil {
		t.Fatalf("reading second recovery state: %v", err)
	}
	second := New()
	second.Folders = InstanceMap[FolderConfig]{"0": cfg}
	second.StateFile = stateFile
	second.recovery = secondState
	second.folders = &Folders{Folders: make(map[string]*Folder)}
	secondRestart := firstRestart.Add(2 * time.Minute)
	second.recoverInterruptedFolders(secondRestart)

	folder = second.folders.Folders[archivePath]
	if folder == nil || folder.Status != EXTRACTED || !folder.Updated.Equal(updated) {
		t.Fatalf("second restart folder = %+v, want extracted at %s", folder, updated)
	}
}

//nolint:funlen // cleanup round-trip is the assertion.
func TestRecoverExtractedFolderRestoresCleanupPaths(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()
	archivePath := filepath.Join(watchPath, "movie.zip")
	extractedPath := filepath.Join(watchPath, "movie", "episode.mkv")
	if err := os.MkdirAll(filepath.Dir(extractedPath), 0o700); err != nil {
		t.Fatalf("creating extracted directory: %v", err)
	}
	if err := os.WriteFile(archivePath, []byte("archive"), 0o600); err != nil {
		t.Fatalf("creating archive: %v", err)
	}
	if err := os.WriteFile(extractedPath, []byte("episode"), 0o600); err != nil {
		t.Fatalf("creating extracted file: %v", err)
	}

	stateFile := filepath.Join(t.TempDir(), defaultStateFile)
	updated := time.Now().UTC().Add(-2 * time.Minute)
	cfg := &FolderConfig{
		Path:        watchPath,
		MoveBack:    true,
		DeleteFiles: true,
		DeleteOrig:  true,
		DeleteAfter: &cnfg.Duration{Duration: time.Minute},
	}

	unpackerr := New()
	unpackerr.Folders = InstanceMap[FolderConfig]{"0": cfg}
	unpackerr.StateFile = stateFile
	unpackerr.folders = &Folders{Folders: map[string]*Folder{
		archivePath: {
			Updated:  updated,
			Status:   EXTRACTED,
			Config:   cfg,
			Files:    []string{extractedPath},
			Archives: xtractr.ArchiveList{watchPath: {archivePath}},
		},
	}}
	unpackerr.recovery = newRecoveryState()
	unpackerr.recoveryTrackFolder(archivePath, cfg, EXTRACTED, updated)

	state, err := readRecoveryState(stateFile)
	if err != nil {
		t.Fatalf("reading recovery state: %v", err)
	}
	if item := state.Folders[archivePath]; item == nil || len(item.Files) != 1 || len(item.Archives) != 1 {
		t.Fatalf("saved cleanup paths = %+v", item)
	}

	restarted := New()
	restarted.Folders = InstanceMap[FolderConfig]{"0": cfg}
	restarted.StateFile = stateFile
	restarted.recovery = state
	restarted.folders = &Folders{Folders: make(map[string]*Folder)}
	now := updated.Add(2 * time.Minute)
	restarted.recoverInterruptedFolders(now)

	folder := restarted.folders.Folders[archivePath]
	if folder == nil || len(folder.Files) != 1 || folder.Files[0] != extractedPath ||
		len(folder.Archives.List()) != 1 || folder.Archives.List()[0] != archivePath {
		t.Fatalf("restored cleanup paths = %+v", folder)
	}

	restarted.checkFolderStats(now)
	deleteFiles := <-restarted.delChan
	deleteOrig := <-restarted.delChan
	if len(deleteFiles.Paths) != 1 || deleteFiles.Paths[0] != extractedPath {
		t.Fatalf("delete-files request = %+v", deleteFiles)
	}
	if len(deleteOrig.Paths) != 1 || deleteOrig.Paths[0] != archivePath {
		t.Fatalf("delete-original request = %+v", deleteOrig)
	}
}

func TestRecoverExtractedFolderDropsOutOfRootCleanupPaths(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()
	archivePath := filepath.Join(watchPath, "movie.zip")
	if err := os.WriteFile(archivePath, []byte("archive"), 0o600); err != nil {
		t.Fatalf("creating archive: %v", err)
	}

	outsideRoot := filepath.Join(filepath.Dir(watchPath), filepath.Base(watchPath)+"-other")
	safeFile := filepath.Join(watchPath, "movie", "episode.mkv")
	unsafeFile := filepath.Join(outsideRoot, "episode.mkv")
	unsafeArchive := filepath.Join(outsideRoot, "movie.zip")
	cfg := &FolderConfig{Path: watchPath, DeleteAfter: &cnfg.Duration{Duration: time.Minute}}
	unpackerr := New()
	unpackerr.Folders = InstanceMap[FolderConfig]{"0": cfg}
	unpackerr.folders = &Folders{Folders: make(map[string]*Folder)}
	unpackerr.recovery = &recoveryState{
		Version: recoveryStateVersion,
		Folders: map[string]*recoveryFolder{
			archivePath: {
				Path:      archivePath,
				WatchPath: watchPath,
				Status:    EXTRACTED.String(),
				Updated:   time.Now().UTC(),
				Files:     []string{safeFile, unsafeFile},
				Archives:  []string{archivePath, unsafeArchive},
			},
		},
	}

	unpackerr.recoverInterruptedFolders(time.Now().UTC())

	folder := unpackerr.folders.Folders[archivePath]
	if folder == nil || len(folder.Files) != 1 || folder.Files[0] != safeFile {
		t.Fatalf("filtered files = %+v", folder)
	}
	if got := folder.Archives.List(); len(got) != 1 || got[0] != archivePath {
		t.Fatalf("filtered archives = %v", got)
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

func TestRecoverLegacyDirectoryItemIsDropped(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()
	legacyDir := filepath.Join(watchPath, "incoming")
	if err := os.Mkdir(legacyDir, 0o700); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	unpackerr := newRecoveryTestUnpackerr(watchPath, legacyDir, WAITING, now.Add(-time.Minute))
	unpackerr.StateFile = filepath.Join(t.TempDir(), defaultStateFile)
	unpackerr.recoverInterruptedFolders(now)

	if unpackerr.folders.Folders[legacyDir] != nil {
		t.Fatalf("legacy directory was restored as work: %s", legacyDir)
	}
	if unpackerr.recovery.Folders[legacyDir] != nil {
		t.Fatalf("legacy directory remained in recovery: %s", legacyDir)
	}
}

func TestRecoverLegacyExtractingDirectoryCleansPartialOutput(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()
	legacyDir := filepath.Join(watchPath, "incoming")
	partial := legacyDir + suffix
	if err := os.Mkdir(legacyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(partial, 0o700); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	unpackerr := newRecoveryTestUnpackerr(watchPath, legacyDir, EXTRACTING, now.Add(-time.Minute))
	unpackerr.StateFile = filepath.Join(t.TempDir(), defaultStateFile)
	unpackerr.recoverInterruptedFolders(now)

	if _, err := os.Stat(partial); !os.IsNotExist(err) {
		t.Fatalf("legacy partial output was not cleaned: %v", err)
	}
}

func newRecoveryTestUnpackerr(watchPath, archivePath string, status ExtractStatus, updated time.Time) *Unpackerr {
	unpackerr := New()
	unpackerr.Folders = InstanceMap[FolderConfig]{"0": {Path: watchPath}}
	unpackerr.folders = &Folders{
		Folders: make(map[string]*Folder),
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
