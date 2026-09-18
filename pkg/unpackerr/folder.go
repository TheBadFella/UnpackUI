package unpackerr

/* Folder extract / callback / cleanup — the watch tracker lives in pkg/folders. */

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/Unpackerr/unpackerr/pkg/folders"
	"golift.io/xtractr"
)

func (u *Unpackerr) validateFolders() error {
	return validateFolderList(u.Folders)
}

func validateFolderList(list InstanceMap[FolderConfig]) error {
	for key, folder := range list {
		if err := validateInstanceSlug(key); err != nil {
			return err
		}

		if folder == nil {
			return errNilConfigEntry
		}

		if strings.TrimSpace(folder.Path) == "" {
			return fmt.Errorf("folder %q: %w", key, folders.ErrNoPath)
		}
	}

	if err := folders.ValidateList(instanceValues(list), parseOptionalMaxBytes); err != nil {
		return fmt.Errorf("validating folders: %w", err)
	}

	return nil
}

// PollFolders begins the routines to watch folders for changes.
// if those changes include the addition of compressed files, they
// are processed for exctraction.
func (u *Unpackerr) PollFolders() {
	// Abs-expand a clone so GET /api/config/folders/live keeps the configured
	// path (file vs env), not the runtime filepath.Abs rewrite.
	watched, _ := folders.Check(folders.CloneList(instanceValues(u.Folders)), u.Logger)
	applyLegacyFolderInterval(watched, u.Folder.Interval.Duration)

	tracker, err := u.Folder.NewWatcher(watched, u.Logger, updateChanBuf, suffix)
	if err != nil {
		u.Errorf("Watching Folders: %s", err)

		return
	}

	u.folders = tracker
	// do not close either watcher.

	if len(watched) == 0 {
		return
	}

	go u.folders.WatchFSNotify()

	if watching := u.folders.FSNotifyPaths(); len(watching) > 0 {
		u.Printf("[Folder] Watching (fsnotify): %s", strings.Join(watching, ", "))
	}

	if summaries := u.folders.PollerSummaries(); len(summaries) > 0 {
		u.folders.StartPollers()
		u.Printf("[Folder] Polling: %s", strings.Join(summaries, ", "))
	}
}

// applyLegacyFolderInterval keeps an explicitly configured global poll interval
// working for old config files and environment overlays. A per-folder interval
// always wins; zero is the only value treated as unset. The new default remains
// fsnotify, including in Docker, when no legacy interval is configured.
func applyLegacyFolderInterval(watched []*FolderConfig, legacy time.Duration) {
	if legacy < folders.MinimumPollInterval {
		return
	}

	for _, cfg := range watched {
		if cfg != nil && cfg.Interval.Duration == 0 {
			cfg.Interval.Duration = legacy
		}
	}
}

// extractTrackedItem starts an archive or folder's extraction after it hasn't been written to in a while.
func (u *Unpackerr) extractTrackedItem(name string, folder *Folder, now time.Time) {
	folder.Updated = now

	if u.deferIncompleteFolderExtract(name, folder, now) {
		return
	}

	exclude := folderExcludeSuffixes(name, folder.Config)
	filter := xtractr.Filter{Path: name, ExcludeSuffix: exclude, AllowSymlinks: folder.Config.AllowSymlinks}
	found := xtractr.FindCompressedFiles(filter)
	if found.Count() == 0 {
		// Keep the folder watched briefly so a late-arriving archive can still be seen,
		// but do not queue, notify, or show this as a completed extraction.
		folder.Status = EXTRACTEDNOTHING
		u.Debugf("[Folder] Ignoring item without compressed files: %s", name)
		u.recoveryTrackFolder(name, folder.Config, folder.Status, now)

		return
	}

	u.folders.Remove(name) // stop the fs watcher(s).
	// update status.
	u.folders.Folders[name].Updated = now
	u.folders.Folders[name].Status = QUEUED

	if u.skipR00Extraction(name, now) {
		u.recoveryClearFolder(name)
		return
	}

	folder.Status = QUEUED

	// create a queue counter in the main history; add to u.Map and send webhook for a new folder.
	u.lockHistory()
	item := u.updateQueueStatus(&newStatus{Name: name, Status: QUEUED}, u.folders.Folders[name].Updated, true)
	u.unlockHistory()
	u.recoveryTrackFolder(name, folder.Config, QUEUED, folder.Updated)

	if folder.Config.MoveBack {
		snap, err := keepDirSnapshot(folder.PreFiles, archiveSnapshotPaths(name, found)...)
		if err != nil {
			u.Errorf("[Folder] Snapshot dests for remnant check: %v", err)
		} else {
			folder.PreFiles = snap
		}
	}

	queueSize, err := u.queueFolderExtract(name, folder, exclude, item)
	if err != nil {
		u.Errorf("[ERROR] %v", err)
		return
	}

	u.Printf("[Folder] Queued: %s, queue size: %d", name, queueSize)
}

func (u *Unpackerr) queueFolderExtract(
	name string, folder *Folder, exclude []string, item *Extract,
) (int, error) {
	queueSize, err := u.Extract(&xtractr.Xtract{
		Password:         u.getPasswordFromPath(name),
		Passwords:        u.Passwords,
		Name:             name,
		Path:             name,
		ExcludeSuffix:    exclude,
		AllowSymlinks:    folder.Config.AllowSymlinks,
		MaxBytes:         folder.Config.ResolvedMaxBytes,
		MaxFiles:         folder.Config.MaxFiles,
		MaxRatio:         folder.Config.MaxRatio,
		MaxNested:        folder.Config.MaxNested,
		ExtrasMaxDepth:   folder.Config.ExtrasMaxDepth,
		TempFolder:       !folder.Config.MoveBack,
		ExtractTo:        folder.Config.ExtractPath,
		DeleteOrig:       false,
		CBChannel:        u.folders.Updates,
		CBFunction:       nil,
		Progress:         u.progressUpdateCallback(item),
		LogFile:          !folder.Config.DisableLog,
		DisableRecursion: folder.Config.DisableRecursion,
	})
	if err != nil {
		return 0, fmt.Errorf("queue folder extraction: %w", err)
	}

	return queueSize, nil
}

func (u *Unpackerr) deferIncompleteFolderExtract(name string, folder *Folder, now time.Time) bool {
	partial := folderIncompleteDownload(name)
	if partial == "" {
		partial = multipartIncompleteSibling(name)
	}
	if partial == "" {
		return false
	}

	folder.Status = WAITING
	u.recoveryTrackFolder(name, folder.Config, folder.Status, now)
	u.Debugf("[Folder] Deferring extraction while download is incomplete: %s (%s)", name, partial)

	return true
}

func multipartIncompleteSibling(path string) string {
	base := strings.ToLower(filepath.Base(path))
	partAt := strings.LastIndex(base, ".part")
	if partAt < 0 || !strings.HasSuffix(base, ".rar") {
		return ""
	}

	prefix := base[:partAt+len(".part")]
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := strings.ToLower(entry.Name())
		if !strings.HasPrefix(name, prefix) || !isIncompleteArchiveName(name) {
			continue
		}

		return filepath.Join(filepath.Dir(path), entry.Name())
	}

	return ""
}

// incompleteDownloadSuffixes are temporary names download clients use before a file is finalized.
func incompleteDownloadSuffixes() []string {
	return []string{".part", ".partial", ".crdownload", ".download", ".aria2", ".!qb"}
}

// folderIncompleteDownload returns the first temporary archive download found below path.
// Download clients rename these files to their final archive name after flushing and
// verifying them, so their presence means the containing folder is not ready to scan.
// Non-archive temps (for example video.mkv.part) are ignored so media folders are not queued.
func folderIncompleteDownload(path string) string {
	var partial string

	_ = filepath.WalkDir(path, func(itemPath string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			return nil
		}

		if !isIncompleteArchiveName(entry.Name()) {
			return nil
		}

		partial = itemPath

		return filepath.SkipAll
	})

	return partial
}

func isIncompleteArchiveName(name string) bool {
	lower := strings.ToLower(name)

	for _, suffix := range incompleteDownloadSuffixes() {
		if !strings.HasSuffix(lower, suffix) {
			continue
		}

		if xtractr.IsArchiveFile(strings.TrimSuffix(lower, suffix)) {
			return true
		}
	}

	return false
}

func folderHasExtractableContent(path string, cfg *FolderConfig) bool {
	if path == "" {
		return false
	}

	if xtractr.IsArchiveFile(path) || isIncompleteArchiveName(filepath.Base(path)) {
		return true
	}

	if folderIncompleteDownload(path) != "" {
		return true
	}

	exclude := folderExcludeSuffixes(path, cfg)

	return xtractr.FindCompressedFiles(xtractr.Filter{Path: path, ExcludeSuffix: exclude}).Count() > 0
}

func folderDerivedOutputPath(path string) string {
	if path == "" || !xtractr.IsArchiveFile(path) {
		return ""
	}

	extensions := xtractr.SupportedExtensions()
	sort.Slice(extensions, func(i, j int) bool {
		return len(extensions[i]) > len(extensions[j])
	})

	lower := strings.ToLower(path)
	for _, ext := range extensions {
		if strings.HasSuffix(lower, strings.ToLower(ext)) {
			return filepath.Clean(path[:len(path)-len(ext)])
		}
	}

	return ""
}

// skipR00Extraction drops a tracked .r00 when a sibling .rar exists (xtractr would extract the rar).
func (u *Unpackerr) skipR00Extraction(name string, now time.Time) bool {
	if !strings.HasSuffix(strings.ToLower(name), ".r00") ||
		!xtractr.CheckR00ForRarFile(getFileList(filepath.Dir(name)), filepath.Base(name)) {
		return false
	}

	u.Printf("[Folder] Removing tracked item without extraction: %v (rar file exists)", name)
	u.folders.Folders[name].Status = EXTRACTEDNOTHING
	u.lockHistory()
	u.updateQueueStatus(&newStatus{Name: name, Status: EXTRACTEDNOTHING}, now, false)
	u.unlockHistory()

	return true
}

// folderExcludeSuffixes returns archive suffixes to ignore when scanning for items to extract.
// For watched archive files with disable_recursion enabled, exclude all archive suffixes so
// extracted nested archives are not picked up by follow-up scans in the extraction library.
func folderExcludeSuffixes(path string, cfg *FolderConfig) []string {
	exclude := []string{}
	if !cfg.ExtractISOs {
		exclude = append(exclude, ".iso")
	}

	if !cfg.DisableRecursion {
		return exclude
	}

	stat, err := os.Stat(path)
	if err != nil || stat.IsDir() || !xtractr.IsArchiveFile(path) {
		return exclude
	}

	return append(exclude, xtractr.SupportedExtensions()...)
}

func getFileList(path string) []os.FileInfo {
	dir, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer dir.Close()

	if stat, err := dir.Stat(); err != nil || !stat.IsDir() {
		return nil
	}

	fileList, err := dir.Readdir(-1)
	if err != nil {
		return nil
	}

	return fileList
}

// folderXtractrCallback is run twice by the xtractr library when the extraction begins, and finishes.
//
//nolint:funlen // The callback owns the full extraction state transition.
func (u *Unpackerr) folderXtractrCallback(resp *xtractr.Response) {
	now := resp.Started.Add(resp.Elapsed)

	u.lockHistory()

	folder, found := u.folders.Folders[resp.X.Name]
	item := u.Map[resp.X.Name]

	if !found || item == nil {
		delete(u.folders.Folders, resp.X.Name)
		delete(u.Map, resp.X.Name)
		u.notifyQueueLocked()
		u.unlockHistory()
		u.recoveryClearFolder(resp.X.Name)

		return
	}

	if !resp.Done {
		resetExtractProgress(item, resp.Archives.Count()+resp.Extras.Count())

		folder.Status = EXTRACTING
		u.Printf("[Folder] Extraction Started: %s, retries: %d, items in queue: %d", resp.X.Name, folder.Retries, resp.Queued)
		folder.Updated = now
		u.updateQueueStatus(&newStatus{Name: resp.X.Name, Resp: resp, Status: folder.Status}, folder.Updated, true)
		u.unlockHistory()
		u.recoveryTrackFolder(resp.X.Name, folder.Config, folder.Status, folder.Updated)

		return
	}

	if errors.Is(resp.Error, xtractr.ErrNoCompressedFiles) {
		folder.Status = EXTRACTEDNOTHING
		u.Printf("[Folder] %s: %s: %v", folder.Status.Desc(), resp.X.Name, resp.Error)
		folder.Updated = now
		u.updateQueueStatus(&newStatus{Name: resp.X.Name, Resp: resp, Status: folder.Status}, folder.Updated, true)
		u.unlockHistory()
		u.recoveryTrackFolder(resp.X.Name, folder.Config, folder.Status, folder.Updated)

		return
	}

	preFiles := folder.PreFiles
	retries := folder.Retries
	configPath := folder.Config.Path

	u.unlockHistory()

	remnantStatus, remnants := u.finishFolderExtractWork(resp, preFiles, retries, configPath)

	u.lockHistory()
	defer u.unlockHistory()

	folder, found = u.folders.Folders[resp.X.Name]
	if !found {
		return
	}

	u.commitFolderExtract(folder, resp, remnantStatus, remnants)
	folder.Updated = now
	u.updateQueueStatus(&newStatus{Name: resp.X.Name, Resp: resp, Status: folder.Status}, folder.Updated, true)
	u.recoveryTrackFolder(resp.X.Name, folder.Config, folder.Status, folder.Updated)
}

// finishFolderExtractWork logs, records metrics, and classifies remnants without
// holding History.mu — remnant cleanup may RemoveAll large trees.
func (u *Unpackerr) finishFolderExtractWork(
	resp *xtractr.Response,
	preFiles map[string]os.FileInfo,
	retries uint,
	configPath string,
) (ExtractStatus, bool) {
	if resp.Error != nil {
		u.Errorf("[Folder] %s: %s: %v", EXTRACTFAILED.Desc(), resp.X.Name, resp.Error)
	} else {
		u.Printf("[Folder] Extraction Finished: %s => elapsed: %v, archives: %d, "+
			"extra archives: %d, files extracted: %d, written: %sB",
			resp.X.Name, resp.Elapsed.Round(time.Second), resp.Archives.Count(),
			resp.Extras.Count(), len(resp.NewFiles), bytefmt.ByteSize(resp.Size))
	}

	u.updateMetrics(resp, FolderString, configPath)

	return u.handleRemnants(resp, preFiles, retries)
}

// commitFolderExtract applies remnant/error/success status under History.mu.
// remnant_action=off sets noRetry so checkFolderStats will not restart.
func (u *Unpackerr) commitFolderExtract(folder *Folder, resp *xtractr.Response, status ExtractStatus, remnants bool) {
	if resp.Error != nil {
		folder.Archives = resp.Archives
	}

	if remnants {
		u.finishFolderRemnants(folder, resp, status)
		return
	}

	if resp.Error != nil {
		folder.Status = EXTRACTFAILED
		return
	}

	folder.Archives = resp.Archives
	folder.Status = EXTRACTED
	folder.Files = resp.NewFiles
}

func (u *Unpackerr) finishFolderRemnants(folder *Folder, resp *xtractr.Response, status ExtractStatus) {
	if status == WAITING {
		u.Printf("[Folder] Cleared interrupted-extraction remnant(s), restarting extraction: %s", resp.X.Name)

		folder.Status = EXTRACTFAILED

		return
	}

	if remnantAction(u.RemnantAction) == "off" {
		folder.NoRetry = true
	}

	u.Errorf("[Folder] Extraction blocked by interrupted-extraction remnant(s): %s", resp.X.Name)

	folder.Status = EXTRACTFAILED
}

// processEvent is here to process the event in the `*Unpackerr` scope before sending it back to the `*Folders` scope.
func (u *Unpackerr) processEvent(event *eventData, now time.Time) {
	if event == nil || event.Config == nil {
		return
	}

	// Do not watch our own log file.
	if event.File == u.LogFile || event.File == u.Webserver.LogFile {
		return
	}

	dirPath := filepath.Clean(event.File)
	if event.File == "" {
		dirPath = filepath.Join(event.Config.Path, event.Name)
	}
	_, trackedBefore := u.folders.Folders[dirPath]

	affected := u.folders.ProcessEvent(event, now)
	if len(affected) == 0 {
		return
	}

	for _, path := range affected {
		u.syncFolderQueue(path)

		if folder := u.folders.Folders[path]; folder != nil {
			if folder.Status == WAITING && !folderHasExtractableContent(path, folder.Config) {
				u.recoveryClearFolder(path)
			} else {
				u.recoveryTrackFolder(path, folder.Config, folder.Status, folder.Updated)
			}
		} else if path == dirPath && trackedBefore {
			u.recoveryClearFolder(path)
		}
	}

	if _, err := os.Stat(dirPath); err != nil {
		u.dropMissingFolderDescendants(dirPath)
	}
}

// scanWatchedFolders mirrors startup discoveries into the UI queue and
// recovery state. Folders.Scan preserves timestamps for recovered tasks.
func (u *Unpackerr) scanWatchedFolders(now time.Time) {
	if u == nil || u.folders == nil {
		return
	}

	for _, path := range u.folders.Scan(now) {
		u.syncFolderQueue(path)
		if folder := u.folders.Folders[path]; folder != nil {
			u.recoveryTrackFolder(path, folder.Config, folder.Status, folder.Updated)
		}
	}
}

// dropMissingFolderDescendants clears per-archive work when a watched
// container directory disappears in one event. The child fs events are not
// guaranteed to arrive after a directory removal.
func (u *Unpackerr) dropMissingFolderDescendants(dirPath string) {
	for name, folder := range u.folders.Folders {
		if name == dirPath || !isDescendantPath(dirPath, name) {
			continue
		}
		// Queued/running work still owns its recovery record until its callback
		// settles. Removing it here would make a crash lose retry cleanup state.
		if folder.Status != WAITING {
			continue
		}

		delete(u.folders.Folders, name)
		u.folders.Remove(name)
		u.syncFolderQueue(name)
		u.recoveryClearFolder(name)
	}
}

func isDescendantPath(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}

	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// syncFolderQueue mirrors a watched folder into the overview queue while it is
// still tracking writes (before start_delay queues extraction).
func (u *Unpackerr) syncFolderQueue(dirPath string) {
	u.lockHistory()
	defer u.unlockHistory()

	folder, ok := u.folders.Folders[dirPath]
	if !ok {
		if item := u.Map[dirPath]; item != nil && item.App == FolderString && item.Status == WAITING {
			delete(u.Map, dirPath)
			u.notifyQueueLocked()
		}

		return
	}

	if folder.Status != WAITING {
		return
	}
	if !folderHasExtractableContent(dirPath, folder.Config) {
		if item := u.Map[dirPath]; item != nil && item.App == FolderString && item.Status == WAITING {
			delete(u.Map, dirPath)
			u.notifyQueueLocked()
		}

		return
	}

	if _, exists := u.Map[dirPath]; !exists {
		u.updateQueueStatus(&newStatus{Name: dirPath, Status: WAITING}, folder.Updated, false)
		return
	}

	item := u.Map[dirPath]
	if item.Status != WAITING {
		return
	}

	item.Updated = folder.Updated
	u.stampQueueDue(dirPath, item)

	if u.hub != nil {
		u.hub.notifyProgress(u.queueFromExtract(dirPath, item))
	}
}

func (u *Unpackerr) checkWaitingFolder(name string, folder *Folder, now time.Time) {
	if _, err := os.Stat(name); err != nil {
		delete(u.folders.Folders, name)
		u.folders.Remove(name)
		u.syncFolderQueue(name)

		return
	}

	if now.Sub(folder.Updated) >= u.StartDelay.Duration {
		u.extractTrackedItem(name, folder, now)
	}
}

// checkFolderStats runs at an interval to see if any folders need work done on them.
// This runs on an interval ticker in the main go routine.
func (u *Unpackerr) checkFolderStats(now time.Time) {
	for name, folder := range u.folders.Folders {
		switch elapsed := now.Sub(folder.Updated); {
		case WAITING == folder.Status:
			u.checkWaitingFolder(name, folder, now)
		case EXTRACTEDNOTHING == folder.Status:
			// Wait until this item hasn't been touched for a while, so it doesn't re-queue.
			if now.Sub(folder.Updated) > u.StartDelay.Duration {
				u.folders.Remove(name)
				u.lockHistory()
				delete(u.Map, name)
				u.notifyQueueLocked()
				u.unlockHistory()
				delete(u.folders.Folders, name)
				u.recoveryClearFolder(name)
			}
		case EXTRACTFAILED == folder.Status:
			u.checkFailedFolder(name, folder, now, elapsed)
		case EXTRACTED == folder.Status && folder.Config.DeleteAfter.Duration <= 0:
			// if DeleteAfter is 0 we don't delete anything. we are done.
			u.lockHistory()
			u.updateQueueStatus(&newStatus{Name: name, Status: DELETED, Resp: nil}, now, false)
			u.unlockHistory()
			u.recoveryClearFolder(name)
			delete(u.folders.Folders, name)
		case EXTRACTED == folder.Status && elapsed >= folder.Config.DeleteAfter.Duration:
			u.deleteAfterReached(name, now, folder)
		}
	}
}

func (u *Unpackerr) checkFailedFolder(name string, folder *Folder, now time.Time, elapsed time.Duration) {
	switch {
	case folder.NoRetry:
		u.lockHistory()
		u.copyFolderRetriesLocked(name, folder)
		u.updateQueueStatus(&newStatus{Name: name, Status: DELETED, Resp: nil}, now, true)
		u.unlockHistory()
		u.recoveryClearFolder(name)
		delete(u.folders.Folders, name)
		u.Printf("[Folder] Remnant left in place (remnant_action=off), giving up: %s", name)
	case elapsed >= u.RetryDelay.Duration && folder.Retries < u.maxRetries():
		folder.Retries++
		folder.Updated = now
		folder.Status = WAITING

		u.lockHistory()
		u.Retries++
		u.copyFolderRetriesLocked(name, folder)

		if item := u.Map[name]; item != nil {
			item.Status = WAITING
			item.Updated = now
		}

		u.notifyQueueLocked()
		u.unlockHistory()
		u.recoveryTrackFolder(name, folder.Config, folder.Status, folder.Updated)
		u.Printf("[Folder] Re-starting Failed Extraction: %s (%d/%d, failed %v ago)",
			folder.Config.Path, folder.Retries, u.maxRetries(), elapsed.Round(time.Second))
	case folder.Retries < u.maxRetries():
		// Still waiting for retry_delay; do not give up yet.
	default:
		// Retries exhausted — clean up to prevent the item from staying in the map forever.
		u.lockHistory()
		u.copyFolderRetriesLocked(name, folder)
		u.updateQueueStatus(&newStatus{Name: name, Status: DELETED, Resp: nil}, now, true)
		u.unlockHistory()
		u.recoveryClearFolder(name)
		delete(u.folders.Folders, name)
		u.Printf("[Folder] Retries exhausted (%d/%d), giving up: %s", folder.Retries, u.maxRetries(), name)
	}
}

//nolint:wsl_v5
func (u *Unpackerr) deleteAfterReached(name string, now time.Time, folder *Folder) {
	var webhook bool
	// Folder reached delete delay (after extraction), nuke it.
	if folder.Config.DeleteFiles && !folder.Config.MoveBack {
		u.queueDelete(&fileDeleteReq{Paths: []string{strings.TrimRight(name, `/\`) + suffix}})
		webhook = true
	} else if folder.Config.DeleteFiles && len(folder.Files) > 0 {
		u.queueDelete(&fileDeleteReq{Paths: folder.Files})
		webhook = true
	}

	if folder.Config.DeleteOrig && !folder.Config.MoveBack {
		u.queueDelete(&fileDeleteReq{Paths: []string{name}})
		webhook = true
	} else if folder.Config.DeleteOrig && len(folder.Archives) > 0 {
		u.queueDelete(&fileDeleteReq{Paths: folder.Archives.List()})
		webhook = true
	}

	u.lockHistory()
	u.updateQueueStatus(&newStatus{Name: name, Status: DELETED, Resp: nil}, now, webhook)
	u.unlockHistory()
	// Folder reached delete delay (after extraction), nuke it.
	u.recoveryClearFolder(name)
	delete(u.folders.Folders, name)
}

type newStatus struct {
	Name   string
	Status ExtractStatus
	Resp   *xtractr.Response
}

// updateQueueStatus for an on-going tracked extraction.
// This is called from a channel callback to update status in a single go routine.
// This is used by apps and Folders in a few other places as well.
func (u *Unpackerr) updateQueueStatus(data *newStatus, now time.Time, sendHook bool) *Extract {
	if _, ok := u.Map[data.Name]; !ok {
		// This is a new Folder entering tracked status.
		// Arr apps do not land here. They create their own queued items in u.Map.
		u.Map[data.Name] = &Extract{
			Path:    data.Name,
			App:     FolderString,
			Status:  data.Status,
			Updated: now,
			IDs:     map[string]any{"title": data.Name}, // required or webhook may break.
			Resp:    data.Resp,
		}

		u.Map[data.Name].XProg = &ExtractProgress{Extract: u.Map[data.Name]}
		u.copyFolderExtractLocked(data.Name, u.Map[data.Name])

		if sendHook {
			u.runAllHooks(u.Map[data.Name])
		}

		u.notifyQueueLocked()

		return u.Map[data.Name]
	}

	if data.Resp != nil {
		u.Map[data.Name].Resp = data.Resp
	}

	u.Map[data.Name].Status = data.Status
	u.Map[data.Name].Updated = now
	u.copyFolderExtractLocked(data.Name, u.Map[data.Name])

	if sendHook {
		u.runAllHooks(u.Map[data.Name])
	}

	u.maybeRecordHistory(data.Name, u.Map[data.Name])
	u.notifyQueueLocked()

	return u.Map[data.Name]
}

// copyFolderExtractLocked copies tracker fields the queue/history Extract
// should own so HTTP snapshots do not read u.folders.Folders. Caller holds
// History.mu.
func (u *Unpackerr) copyFolderExtractLocked(name string, item *Extract) {
	if item == nil || item.App != FolderString || u.folders == nil {
		return
	}

	folder, ok := u.folders.Folders[name]
	if !ok {
		return
	}

	item.Retries = folder.Retries
	item.NoRetry = folder.NoRetry

	if folder.PreFiles != nil {
		item.PreFiles = folder.PreFiles
	}

	if folder.Config != nil && folder.Config.DeleteAfter != nil {
		item.DeleteDelay = folder.Config.DeleteAfter.Duration
	}
}

// copyFolderRetriesLocked writes the folder retry count onto the queue/history
// Extract. Logs and stats already increment folder.Retries; /api/history reads
// Extract.Retries. Caller holds History.mu.
func (u *Unpackerr) copyFolderRetriesLocked(name string, folder *Folder) {
	if item := u.Map[name]; item != nil {
		item.Retries = folder.Retries
	}
}
