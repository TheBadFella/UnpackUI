package unpackerr

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golift.io/xtractr"
)

type noopLogger struct{}

func (noopLogger) Printf(string, ...any) {}
func (noopLogger) Errorf(string, ...any) {}
func (noopLogger) Debugf(string, ...any) {}

func TestNormalizeFolderExcludePaths(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	relative := "permanent"
	absolute := filepath.Join(base, "keep")

	paths := normalizeFolderExcludePaths(base, []string{"", "  ", relative, absolute})
	if len(paths) != 2 {
		t.Fatalf("expected 2 normalized paths, got %d: %v", len(paths), paths)
	}

	if paths[0] != filepath.Join(base, relative) {
		t.Fatalf("unexpected relative path normalization: %q", paths[0])
	}

	if paths[1] != absolute {
		t.Fatalf("unexpected absolute path normalization: %q", paths[1])
	}
}

func TestFolderConfigIsExcludedPath(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	excluded := filepath.Join(base, "permanent")
	cfg := &FolderConfig{ExcludePaths: []string{excluded}}

	if !cfg.isExcludedPath(excluded) {
		t.Fatal("expected exact excluded path to match")
	}

	if !cfg.isExcludedPath(filepath.Join(excluded, "sub", "file.rar")) {
		t.Fatal("expected child path of excluded folder to match")
	}

	if cfg.isExcludedPath(excluded + "_other") {
		t.Fatal("did not expect prefix-only sibling path to match")
	}
}

func TestFoldersProcessEventCurrentBehavior(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()
	cfg := &FolderConfig{Path: watchPath}
	folders := newTestFolders(t, cfg)

	archive := filepath.Join(watchPath, "movie.rar")
	if err := os.WriteFile(archive, []byte("x"), 0o600); err != nil {
		t.Fatalf("creating archive test file: %v", err)
	}

	folders.processEvent(&eventData{
		cnfg: cfg,
		name: filepath.Base(archive),
		file: archive,
		op:   "test",
	}, time.Now())

	if _, ok := folders.Folders[archive]; !ok {
		t.Fatalf("expected archive path to be tracked: %s", archive)
	}

	plain := filepath.Join(watchPath, "note.txt")
	if err := os.WriteFile(plain, []byte("x"), 0o600); err != nil {
		t.Fatalf("creating non-archive test file: %v", err)
	}

	folders.processEvent(&eventData{
		cnfg: cfg,
		name: filepath.Base(plain),
		file: plain,
		op:   "test",
	}, time.Now())

	if _, ok := folders.Folders[plain]; ok {
		t.Fatalf("did not expect non-archive file to be tracked: %s", plain)
	}

	dir := filepath.Join(watchPath, "incoming")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatalf("creating folder test dir: %v", err)
	}

	folders.processEvent(&eventData{
		cnfg: cfg,
		name: filepath.Base(dir),
		file: dir,
		op:   "test",
	}, time.Now())

	if _, ok := folders.Folders[dir]; !ok {
		t.Fatalf("expected folder path to be tracked: %s", dir)
	}
}

func TestExtractTrackedItemWithoutArchivesSkipsQueue(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()
	itemPath := filepath.Join(watchPath, "movie")
	if err := os.Mkdir(itemPath, 0o700); err != nil {
		t.Fatalf("creating watched item: %v", err)
	}
	if err := os.WriteFile(filepath.Join(itemPath, "movie.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatalf("creating media file: %v", err)
	}

	now := time.Now()
	cfg := &FolderConfig{Path: watchPath}
	folder := &Folder{updated: now.Add(-time.Minute), status: WAITING, config: cfg}
	unpackerr := New()
	unpackerr.KeepHistory = 0
	unpackerr.folders = &Folders{
		Logs:    unpackerr.Logger,
		Folders: map[string]*Folder{itemPath: folder},
		Outputs: make(map[string]string),
	}

	// Xtractr is intentionally nil: reaching the worker queue would panic.
	unpackerr.extractTrackedItem(itemPath, folder, now)

	if folder.status != EXTRACTEDNOTHING {
		t.Fatalf("expected archive-free folder status %s, got %s", EXTRACTEDNOTHING, folder.status)
	}
	if item := unpackerr.Map[itemPath]; item != nil {
		t.Fatalf("expected archive-free folder to stay out of the UI queue, got %+v", item)
	}
	if unpackerr.folders.Folders[itemPath] != folder {
		t.Fatal("expected archive-free folder to remain tracked briefly to avoid re-queue")
	}
	if len(unpackerr.Items) != 0 {
		t.Fatalf("expected archive-free folder not to enter queue history, got %v", unpackerr.Items)
	}
}

func TestIncompleteArchiveName(t *testing.T) {
	t.Parallel()

	if !isIncompleteArchiveName("show.7z.part") {
		t.Fatal("expected archive partial to count as incomplete")
	}
	if !isIncompleteArchiveName("movie.zip.crdownload") {
		t.Fatal("expected browser archive download to count as incomplete")
	}
	if isIncompleteArchiveName("episode.mkv.part") {
		t.Fatal("did not expect media partial to count as an incomplete archive")
	}
	if isIncompleteArchiveName("episode.mkv") {
		t.Fatal("did not expect finished media file to count as an incomplete archive")
	}
}

func TestFoldersProcessEventTracksMediaFolderWithoutQueueing(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()
	cfg := &FolderConfig{Path: watchPath}
	folders := newTestFolders(t, cfg)

	dir := filepath.Join(watchPath, "episode")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatalf("creating media folder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "episode.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatalf("creating media file: %v", err)
	}

	folders.processEvent(&eventData{
		cnfg: cfg,
		name: filepath.Base(dir),
		file: filepath.Join(dir, "episode.mkv"),
		op:   "test",
	}, time.Now())

	if _, ok := folders.Folders[dir]; !ok {
		t.Fatalf("expected media folder to stay watched: %s", dir)
	}
}

func TestBuildWebStateOmitsMediaOnlyWatchedFolders(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()
	itemPath := filepath.Join(watchPath, "episode")
	if err := os.Mkdir(itemPath, 0o700); err != nil {
		t.Fatalf("creating media folder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(itemPath, "episode.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatalf("creating media file: %v", err)
	}

	now := time.Now()
	unpackerr := New()
	unpackerr.folders = &Folders{
		Logs: unpackerr.Logger,
		Folders: map[string]*Folder{
			itemPath: {updated: now, status: WAITING, config: &FolderConfig{Path: watchPath}},
		},
	}

	snapshot := unpackerr.buildWebState(now)
	if len(snapshot.Items) != 0 {
		t.Fatalf("expected media-only watched folder to stay out of the UI, got %+v", snapshot.Items)
	}
}

func TestSaveEventResetsExtractedNothingWhenFolderChanges(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()
	cfg := &FolderConfig{Path: watchPath}
	folders := newTestFolders(t, cfg)
	dir := filepath.Join(watchPath, "show")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatalf("creating folder: %v", err)
	}

	folders.Folders[dir] = &Folder{updated: time.Now().Add(-time.Minute), status: EXTRACTEDNOTHING, config: cfg}
	folders.processEvent(&eventData{
		cnfg: cfg,
		name: filepath.Base(dir),
		file: dir,
		op:   "test",
	}, time.Now())

	if folders.Folders[dir].status != WAITING {
		t.Fatalf("expected later folder activity to wait for extraction again, got %s", folders.Folders[dir].status)
	}
}

func TestExtractTrackedItemDefersIncompleteDownloadUntilArchiveFinalizes(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()
	itemPath := filepath.Join(watchPath, "show")
	if err := os.Mkdir(itemPath, 0o700); err != nil {
		t.Fatalf("creating watched item: %v", err)
	}

	partialPath := filepath.Join(itemPath, "show.7z.part")
	if err := os.WriteFile(partialPath, []byte("archive"), 0o600); err != nil {
		t.Fatalf("creating partial archive: %v", err)
	}

	now := time.Now()
	cfg := &FolderConfig{Path: watchPath}
	folder := &Folder{updated: now.Add(-time.Minute), status: WAITING, config: cfg}
	unpackerr := New()
	unpackerr.KeepHistory = 0
	unpackerr.folders = &Folders{
		Logs:    unpackerr.Logger,
		Folders: map[string]*Folder{itemPath: folder},
		Outputs: make(map[string]string),
		Updates: make(chan *xtractr.Response, updateChanBuf),
	}

	unpackerr.extractTrackedItem(itemPath, folder, now)

	if folder.status != WAITING {
		t.Fatalf("expected partial download to remain %s, got %s", WAITING, folder.status)
	}
	if !folder.updated.Equal(now) {
		t.Fatalf("expected deferred item timestamp %v, got %v", now, folder.updated)
	}
	if unpackerr.Map[itemPath] != nil {
		t.Fatalf("expected partial download to stay out of completed history, got %+v", unpackerr.Map[itemPath])
	}
	if unpackerr.folders.Folders[itemPath] != folder {
		t.Fatal("expected partial download to remain tracked")
	}

	archivePath := filepath.Join(itemPath, "show.7z")
	if err := os.Rename(partialPath, archivePath); err != nil {
		t.Fatalf("finalizing partial archive: %v", err)
	}

	unpackerr.Xtractr = xtractr.NewQueue(&xtractr.Config{Parallel: 1})
	t.Cleanup(func() { unpackerr.Stop() })
	unpackerr.extractTrackedItem(itemPath, folder, now.Add(time.Minute))

	if folder.status != QUEUED {
		t.Fatalf("expected finalized archive status %s, got %s", QUEUED, folder.status)
	}
	if item := unpackerr.Map[itemPath]; item == nil || item.Status != QUEUED {
		t.Fatalf("expected finalized archive in extraction queue, got %+v", item)
	}
}

func TestExtractTrackedItemWithArchiveQueuesExtraction(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()
	itemPath := filepath.Join(watchPath, "movie")
	if err := os.Mkdir(itemPath, 0o700); err != nil {
		t.Fatalf("creating watched item: %v", err)
	}
	if err := os.WriteFile(filepath.Join(itemPath, "movie.zip"), []byte("archive"), 0o600); err != nil {
		t.Fatalf("creating archive file: %v", err)
	}

	now := time.Now()
	cfg := &FolderConfig{Path: watchPath}
	folder := &Folder{updated: now.Add(-time.Minute), status: WAITING, config: cfg}
	unpackerr := New()
	unpackerr.KeepHistory = 0
	unpackerr.folders = &Folders{
		Logs:    unpackerr.Logger,
		Folders: map[string]*Folder{itemPath: folder},
		Outputs: make(map[string]string),
		Updates: make(chan *xtractr.Response, updateChanBuf),
	}
	unpackerr.Xtractr = xtractr.NewQueue(&xtractr.Config{Parallel: 1})
	t.Cleanup(func() { unpackerr.Stop() })

	unpackerr.extractTrackedItem(itemPath, folder, now)

	if folder.status != QUEUED {
		t.Fatalf("expected folder with archive status %s, got %s", QUEUED, folder.status)
	}
	item := unpackerr.Map[itemPath]
	if item == nil || item.Status != QUEUED {
		t.Fatalf("expected folder with archive in extraction queue, got %+v", item)
	}
}

func TestFoldersProcessEventExcludedPath(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()

	excluded := filepath.Join(watchPath, "permanent")
	if err := os.MkdirAll(filepath.Join(excluded, "sub"), 0o700); err != nil {
		t.Fatalf("creating excluded test path: %v", err)
	}

	nested := filepath.Join(excluded, "sub", "file.rar")
	if err := os.WriteFile(nested, []byte("x"), 0o600); err != nil {
		t.Fatalf("creating nested archive file: %v", err)
	}

	cfg := &FolderConfig{
		Path:         watchPath,
		ExcludePaths: []string{excluded},
	}
	folders := newTestFolders(t, cfg)

	// Direct excluded folder.
	folders.processEvent(&eventData{
		cnfg: cfg,
		name: "permanent",
		file: excluded,
		op:   "test",
	}, time.Now())

	if len(folders.Folders) != 0 {
		t.Fatalf("expected no tracked folders for excluded path, got: %v", folders.Folders)
	}

	// Nested event from an excluded folder should also be ignored.
	folders.processEvent(&eventData{
		cnfg: cfg,
		name: "sub",
		file: nested,
		op:   "test",
	}, time.Now())

	if len(folders.Folders) != 0 {
		t.Fatalf("expected no tracked folders for nested excluded event, got: %v", folders.Folders)
	}
}

func TestFoldersHandleFileEventExcludedPath(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()
	excluded := filepath.Join(watchPath, "permanent")

	folders := &Folders{
		Config: []*FolderConfig{{
			Path:         watchPath,
			ExcludePaths: []string{excluded},
		}},
		Events: make(chan *eventData, 1),
		Logs:   noopLogger{},
	}

	folders.handleFileEvent(filepath.Join(excluded, "file.rar"), "test")

	select {
	case event := <-folders.Events:
		t.Fatalf("did not expect event for excluded path: %+v", event)
	default:
	}
}

func newTestFolders(t *testing.T, cfg *FolderConfig) *Folders {
	t.Helper()

	folders, err := (FoldersConfig{Buffer: 32}).newWatcher([]*FolderConfig{cfg}, noopLogger{})
	if err != nil {
		t.Fatalf("creating watcher: %v", err)
	}

	t.Cleanup(func() {
		if folders.Watcher != nil {
			folders.Watcher.Close()
		}

		if folders.FSNotify != nil {
			_ = folders.FSNotify.Close()
		}
	})

	return folders
}
