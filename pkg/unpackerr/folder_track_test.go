package unpackerr

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golift.io/cnfg"
)

func TestApplyLegacyFolderIntervalKeepsPerFolderOverrides(t *testing.T) {
	t.Parallel()

	watched := []*FolderConfig{
		{Path: "/unset"},
		{Path: "/explicit", Interval: cnfg.Duration{Duration: 2 * time.Second}},
		{Path: "/disabled", Interval: cnfg.Duration{Duration: time.Millisecond}},
	}

	applyLegacyFolderInterval(watched, time.Second)

	if got := watched[0].Interval.Duration; got != time.Second {
		t.Fatalf("unset interval = %v, want 1s", got)
	}
	if got := watched[1].Interval.Duration; got != 2*time.Second {
		t.Fatalf("explicit interval = %v, want 2s", got)
	}
	if got := watched[2].Interval.Duration; got != time.Millisecond {
		t.Fatalf("disabled interval = %v, want 1ms", got)
	}
}

func TestFolderWaitingShowsInQueue(t *testing.T) {
	t.Parallel()

	watch := t.TempDir()
	cfg := &FolderConfig{Path: watch}
	unpack := New()

	unpack.Folder.Buffer = 32

	tracker, err := unpack.Folder.NewWatcher([]*FolderConfig{cfg}, unpack.Logger, updateChanBuf, suffix)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(tracker.Close)

	unpack.folders = tracker

	archive := filepath.Join(watch, "movie.rar")
	if err := os.WriteFile(archive, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	first := time.Now().Add(-time.Minute)
	unpack.processEvent(&eventData{Config: cfg, Name: "movie.rar", File: archive, Op: "test"}, first)

	item := unpack.Map[archive]
	if item == nil || item.Status != WAITING || item.App != FolderString {
		t.Fatalf("queue item %+v", item)
	}

	if got := unpack.queueFromExtract(archive, item); got.Progress != "last write" {
		t.Fatalf("progress %q", got.Progress)
	}

	later := first.Add(30 * time.Second)
	unpack.processEvent(&eventData{Config: cfg, Name: "movie.rar", File: archive, Op: "write"}, later)

	if !unpack.Map[archive].Updated.Equal(unpack.folders.Folders[archive].Updated) {
		t.Fatalf("last write %v folder %v", unpack.Map[archive].Updated, unpack.folders.Folders[archive].Updated)
	}

	if err := os.Remove(archive); err != nil {
		t.Fatal(err)
	}

	unpack.processEvent(&eventData{Config: cfg, Name: "movie.rar", File: archive, Op: "remove"}, later.Add(time.Second))

	if unpack.Map[archive] != nil {
		t.Fatal("waiting item still in queue after delete")
	}
}

func TestMediaOnlyFolderStaysOutOfQueueAndRecovery(t *testing.T) {
	t.Parallel()

	watch := t.TempDir()
	cfg := &FolderConfig{Path: watch}
	unpack := New()
	unpack.Folder.Buffer = 32
	unpack.StateFile = filepath.Join(t.TempDir(), defaultStateFile)
	unpack.recovery = newRecoveryState()

	tracker, err := unpack.Folder.NewWatcher([]*FolderConfig{cfg}, unpack.Logger, updateChanBuf, suffix)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(tracker.Close)

	unpack.folders = tracker
	itemPath := filepath.Join(watch, "episode")
	if err := os.Mkdir(itemPath, 0o700); err != nil {
		t.Fatal(err)
	}

	mediaPath := filepath.Join(itemPath, "episode.mkv")
	if err := os.WriteFile(mediaPath, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}

	unpack.processEvent(&eventData{Config: cfg, Name: "episode", File: mediaPath, Op: "test"}, time.Now())

	if item := unpack.Map[itemPath]; item != nil {
		t.Fatalf("media-only folder entered waiting queue: %+v", item)
	}
	if item := unpack.recovery.Folders[itemPath]; item != nil {
		t.Fatalf("media-only folder entered recovery state: %+v", item)
	}
	if _, err := os.Stat(unpack.StateFile); !os.IsNotExist(err) {
		t.Fatalf("media-only folder wrote recovery file: %v", err)
	}
}

func TestPrepopulatedFolderArchivesEnterQueueAndRecoverySeparately(t *testing.T) {
	t.Parallel()

	watch := t.TempDir()
	cfg := &FolderConfig{Path: watch}
	unpack := New()
	unpack.Folder.Buffer = 32
	unpack.StateFile = filepath.Join(t.TempDir(), defaultStateFile)
	unpack.recovery = newRecoveryState()

	tracker, err := unpack.Folder.NewWatcher([]*FolderConfig{cfg}, unpack.Logger, updateChanBuf, suffix)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tracker.Close)
	unpack.folders = tracker

	incoming := filepath.Join(watch, "incoming")
	nested := filepath.Join(incoming, "nested")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	archives := []string{
		filepath.Join(incoming, "one.zip"),
		filepath.Join(incoming, "two.zip"),
		filepath.Join(incoming, "three.zip"),
		filepath.Join(incoming, "four.zip"),
		filepath.Join(nested, "five.zip"),
	}
	for _, archive := range archives {
		if err := os.WriteFile(archive, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	unpack.scanWatchedFolders(time.Now())

	if unpack.Map[incoming] != nil || unpack.recovery.Folders[incoming] != nil {
		t.Fatalf("watch container entered queue or recovery: %+v %+v",
			unpack.Map[incoming], unpack.recovery.Folders[incoming])
	}
	for _, archive := range archives {
		item := unpack.Map[archive]
		if item == nil || item.Status != WAITING || item.App != FolderString {
			t.Fatalf("archive queue item %s: %+v", archive, item)
		}
		if item := unpack.recovery.Folders[archive]; item == nil || item.Status != WAITING.String() {
			t.Fatalf("archive recovery item %s: %+v",
				archive, item)
		}
	}
}

func TestCheckFolderStatsDropsMissingWaiting(t *testing.T) {
	t.Parallel()

	watch := t.TempDir()
	cfg := &FolderConfig{Path: watch}
	unpack := New()
	unpack.Folder.Buffer = 32

	tracker, err := unpack.Folder.NewWatcher([]*FolderConfig{cfg}, unpack.Logger, updateChanBuf, suffix)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(tracker.Close)

	unpack.folders = tracker

	archive := filepath.Join(watch, "movie.rar")
	if err := os.WriteFile(archive, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	unpack.processEvent(&eventData{Config: cfg, Name: "movie.rar", File: archive, Op: "test"}, time.Now())

	if unpack.Map[archive] == nil {
		t.Fatal("expected waiting queue item")
	}

	if err := os.Remove(archive); err != nil {
		t.Fatal(err)
	}

	unpack.checkFolderStats(time.Now())

	if unpack.Map[archive] != nil {
		t.Fatal("waiting item still in queue after checkFolderStats")
	}

	if _, ok := unpack.folders.Folders[archive]; ok {
		t.Fatal("folder still tracked after checkFolderStats")
	}
}

func TestCheckFolderStatsCopiesRetriesToHistory(t *testing.T) {
	t.Parallel()

	const name = "/watch/corrupt"

	unpack := New()
	unpack.MaxRetries = 1
	unpack.RetryDelay.Duration = time.Second
	unpack.KeepHistory = 10
	unpack.histPath = filepath.Join(t.TempDir(), historyFileName)

	now := time.Now()
	failedAt := now.Add(-time.Minute)
	unpack.folders.Folders[name] = &Folder{
		Status:  EXTRACTFAILED,
		Retries: 0,
		Updated: failedAt,
		Config:  &FolderConfig{Path: name},
	}
	unpack.Map[name] = &Extract{
		App:     FolderString,
		Path:    name,
		Status:  EXTRACTFAILED,
		Retries: 0,
		Updated: failedAt,
	}

	unpack.checkFolderStats(now)

	item := unpack.Map[name]
	if item == nil || item.Retries != 1 || item.Status != WAITING {
		t.Fatalf("retry copy %+v", item)
	}

	if unpack.Retries != 1 {
		t.Fatalf("stats retries %d", unpack.Retries)
	}

	folder := unpack.folders.Folders[name]
	if folder == nil || folder.Retries != 1 || folder.Status != WAITING {
		t.Fatalf("folder retry %+v", folder)
	}

	folder.Status = EXTRACTFAILED
	folder.Updated = failedAt
	item.Status = EXTRACTFAILED

	unpack.checkFolderStats(now.Add(time.Minute))

	if _, ok := unpack.folders.Folders[name]; ok {
		t.Fatal("exhausted folder still tracked")
	}

	got := unpack.historySnapshot()
	if len(got) != 1 || got[0].Retries != 1 || got[0].Status != DELETED {
		t.Fatalf("history %+v", got)
	}
}
