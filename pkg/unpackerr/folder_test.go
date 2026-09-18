package unpackerr

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golift.io/xtractr"
)

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

func TestMultipartIncompleteSibling(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	primary := filepath.Join(dir, "release.part01.rar")
	partial := filepath.Join(dir, "release.part02.rar.part")
	if err := os.WriteFile(primary, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(partial, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := multipartIncompleteSibling(primary); got != partial {
		t.Fatalf("incomplete companion = %q, want %q", got, partial)
	}
	if err := os.Rename(partial, filepath.Join(dir, "release.part02.rar")); err != nil {
		t.Fatal(err)
	}
	if got := multipartIncompleteSibling(primary); got != "" {
		t.Fatalf("finished companion reported incomplete: %q", got)
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
	folder := &Folder{Updated: now.Add(-time.Minute), Status: WAITING, Config: cfg}
	unpackerr := New()
	unpackerr.KeepHistory = 0
	unpackerr.folders = &Folders{
		Logs:    unpackerr.Logger,
		Folders: map[string]*Folder{itemPath: folder},
	}

	// Xtractr is intentionally nil: reaching the worker queue would panic.
	unpackerr.extractTrackedItem(itemPath, folder, now)

	if folder.Status != EXTRACTEDNOTHING {
		t.Fatalf("expected archive-free folder status %s, got %s", EXTRACTEDNOTHING, folder.Status)
	}
	if item := unpackerr.Map[itemPath]; item != nil {
		t.Fatalf("expected archive-free folder to stay out of the UI queue, got %+v", item)
	}
	if unpackerr.folders.Folders[itemPath] != folder {
		t.Fatal("expected archive-free folder to remain tracked briefly to avoid re-queue")
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
	folder := &Folder{Updated: now.Add(-time.Minute), Status: WAITING, Config: cfg}
	unpackerr := New()
	unpackerr.KeepHistory = 0
	unpackerr.folders = &Folders{
		Logs:    unpackerr.Logger,
		Folders: map[string]*Folder{itemPath: folder},
		Updates: make(chan *xtractr.Response, updateChanBuf),
	}

	unpackerr.extractTrackedItem(itemPath, folder, now)

	if folder.Status != WAITING {
		t.Fatalf("expected partial download to remain %s, got %s", WAITING, folder.Status)
	}
	if !folder.Updated.Equal(now) {
		t.Fatalf("expected deferred item timestamp %v, got %v", now, folder.Updated)
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

	if folder.Status != QUEUED {
		t.Fatalf("expected finalized archive status %s, got %s", QUEUED, folder.Status)
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
	folder := &Folder{Updated: now.Add(-time.Minute), Status: WAITING, Config: cfg}
	unpackerr := New()
	unpackerr.KeepHistory = 0
	unpackerr.folders = &Folders{
		Logs:    unpackerr.Logger,
		Folders: map[string]*Folder{itemPath: folder},
		Updates: make(chan *xtractr.Response, updateChanBuf),
	}
	unpackerr.Xtractr = xtractr.NewQueue(&xtractr.Config{Parallel: 1})
	t.Cleanup(func() { unpackerr.Stop() })

	unpackerr.extractTrackedItem(itemPath, folder, now)

	if folder.Status != QUEUED {
		t.Fatalf("expected folder with archive status %s, got %s", QUEUED, folder.Status)
	}
	item := unpackerr.Map[itemPath]
	if item == nil || item.Status != QUEUED {
		t.Fatalf("expected folder with archive in extraction queue, got %+v", item)
	}
}
