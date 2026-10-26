package unpackerr

import (
	"os"
	"path/filepath"
	"slices"
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

func TestExpandExtractedFiles(t *testing.T) {
	t.Parallel()

	// 1. Nil or empty slice.
	if got := expandExtractedFiles(nil); len(got) != 0 {
		t.Fatalf("expected empty for nil, got %v", got)
	}
	if got := expandExtractedFiles([]string{}); len(got) != 0 {
		t.Fatalf("expected empty for empty slice, got %v", got)
	}

	// 2. Non-existent paths are preserved.
	missing := []string{filepath.Join("non", "existent", "file.mkv"), filepath.Join("another", "missing.rar")}
	if got := expandExtractedFiles(missing); len(got) != 2 || got[0] != missing[0] || got[1] != missing[1] {
		t.Fatalf("expected non-existent paths preserved, got %v", got)
	}

	// 3. Regular file is preserved and deduplicated.
	dir := t.TempDir()
	f1 := filepath.Join(dir, "file1.mkv")
	if err := os.WriteFile(f1, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := expandExtractedFiles([]string{f1, f1}); len(got) != 1 || got[0] != f1 {
		t.Fatalf("expected deduplicated regular file, got %v", got)
	}

	// 4. Directory with nested files is expanded into sorted leaf files.
	subDir := filepath.Join(dir, "Season 01")
	if err := os.MkdirAll(subDir, 0o750); err != nil {
		t.Fatal(err)
	}
	ep1 := filepath.Join(subDir, "S01E01.mkv")
	ep2 := filepath.Join(subDir, "S01E02.mkv")
	poster := filepath.Join(dir, "poster.jpg")
	if err := os.WriteFile(ep1, []byte("1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ep2, []byte("2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(poster, []byte("p"), 0o600); err != nil {
		t.Fatal(err)
	}

	expanded := expandExtractedFiles([]string{dir})
	want := []string{ep1, ep2, poster, f1}
	slices.Sort(want)
	if len(expanded) != len(want) {
		t.Fatalf("expected %d leaf files, got %d: %v", len(want), len(expanded), expanded)
	}
	for i, p := range want {
		if expanded[i] != p {
			t.Errorf("at index %d: want %s, got %s", i, p, expanded[i])
		}
	}

	// 5. Empty directory is preserved.
	emptyDir := filepath.Join(t.TempDir(), "empty")
	if err := os.MkdirAll(emptyDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if got := expandExtractedFiles([]string{emptyDir}); len(got) != 1 || got[0] != emptyDir {
		t.Fatalf("expected empty directory path preserved, got %v", got)
	}
}

func TestDeleteAfterReachedPurgesFilesAndFolder(t *testing.T) {
	t.Parallel()

	watchPath := t.TempDir()
	releasePath := filepath.Join(watchPath, "Release.Name")
	subPath := filepath.Join(releasePath, "Season 1")
	if err := os.MkdirAll(subPath, 0o750); err != nil {
		t.Fatal(err)
	}

	archive := filepath.Join(releasePath, "release.zip")
	if err := os.WriteFile(archive, []byte("zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	video := filepath.Join(subPath, "episode.mkv")
	if err := os.WriteFile(video, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}

	u := New()
	cfg := &FolderConfig{
		Path:        watchPath,
		DeleteFiles: true,
		DeleteOrig:  true,
	}
	folder := &Folder{
		Config:   cfg,
		Files:    []string{video},
		Archives: xtractr.ArchiveList{releasePath: {archive}},
		Status:   EXTRACTED,
	}
	u.folders = &Folders{
		Logs:    u.Logger,
		Folders: map[string]*Folder{releasePath: folder},
	}
	u.Xtractr = xtractr.NewQueue(&xtractr.Config{Parallel: 1})
	t.Cleanup(func() { u.Stop() })

	now := time.Now()
	u.deleteAfterReached(releasePath, now, folder)

	// Process queued deletes.
	close(u.delChan)
	for req := range u.delChan {
		u.deleteRequest(req)
	}

	// The video file, subfolder, and release folder must be purged.
	if _, err := os.Stat(video); !os.IsNotExist(err) {
		t.Errorf("expected video file deleted, err = %v", err)
	}
	if _, err := os.Stat(subPath); !os.IsNotExist(err) {
		t.Errorf("expected subfolder purged, err = %v", err)
	}
	if _, err := os.Stat(releasePath); !os.IsNotExist(err) {
		t.Errorf("expected release folder purged, err = %v", err)
	}
	// The watched root must NOT be deleted.
	if info, err := os.Stat(watchPath); err != nil || !info.IsDir() {
		t.Errorf("expected watched root intact, err = %v", err)
	}
}
