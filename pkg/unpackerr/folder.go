package unpackerr

/* Folder Watching Codez */

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/fsnotify/fsnotify"
	"github.com/radovskyb/watcher"
	"golift.io/cnfg"
	"golift.io/xtractr"
)

// defaultPollInterval is used if Docker is detected.
const (
	defaultPollInterval = time.Second
	minimumPollInterval = 5 * time.Millisecond
	defaultFolderDelete = 10 * time.Minute
)

// FolderConfig defines the input data for a watched folder.
//
//nolint:lll
type FolderConfig struct {
	DeleteOrig       bool           `json:"delete_original"  toml:"delete_original"   xml:"delete_original"   yaml:"delete_original"`
	DeleteFiles      bool           `json:"delete_files"     toml:"delete_files"      xml:"delete_files"      yaml:"delete_files"`
	DisableLog       bool           `json:"disable_log"      toml:"disable_log"       xml:"disable_log"       yaml:"disable_log"`
	MoveBack         bool           `json:"move_back"        toml:"move_back"         xml:"move_back"         yaml:"move_back"`
	DeleteAfter      *cnfg.Duration `json:"delete_after"     toml:"delete_after"      xml:"delete_after"      yaml:"delete_after"`
	ExtractPath      string         `json:"extract_path"     toml:"extract_path"      xml:"extract_path"      yaml:"extract_path"`
	ExtractISOs      bool           `json:"extract_isos"     toml:"extract_isos"      xml:"extract_isos"      yaml:"extract_isos"`
	DisableRecursion bool           `json:"disableRecursion" toml:"disable_recursion" xml:"disable_recursion" yaml:"disableRecursion"`
	MaxNested        int            `json:"maxNested"        toml:"max_nested"        xml:"max_nested"        yaml:"maxNested"`
	ExtrasMaxDepth   int            `json:"extrasMaxDepth"   toml:"extras_max_depth"  xml:"extras_max_depth"  yaml:"extrasMaxDepth"`
	AllowSymlinks    bool           `json:"allowSymlinks"    toml:"allow_symlinks"    xml:"allow_symlinks"    yaml:"allowSymlinks"`
	MaxBytes         string         `json:"maxBytes"         toml:"max_bytes"         xml:"max_bytes"         yaml:"maxBytes"`
	MaxFiles         int            `json:"maxFiles"         toml:"max_files"         xml:"max_files"         yaml:"maxFiles"`
	MaxRatio         float64        `json:"maxRatio"         toml:"max_ratio"         xml:"max_ratio"         yaml:"maxRatio"`
	// maxBytes is 0 when unset: folder watcher is uncapped.
	maxBytes     uint64
	ExcludePaths []string `json:"exclude_paths" toml:"exclude_paths" xml:"exclude_path" yaml:"exclude_paths"`
	Path         string   `json:"path"          toml:"path"          xml:"path"         yaml:"path"`
}

// Folders holds all known (created) folders in all watch paths.
type Folders struct {
	Logs
	Interval time.Duration
	Config   []*FolderConfig
	Folders  map[string]*Folder
	Outputs  map[string]string
	Events   chan *eventData
	Updates  chan *xtractr.Response
	FSNotify *fsnotify.Watcher
	Watcher  *watcher.Watcher
}

// Logs interface for folders.
type Logs interface {
	Printf(msg string, v ...any)
	Errorf(msg string, v ...any)
	Debugf(msg string, v ...any)
}

// Folder is a "new" watched folder.
type Folder struct {
	updated  time.Time
	status   ExtractStatus
	config   *FolderConfig
	files    []string
	retries  uint
	archives xtractr.ArchiveList
	// preFiles is the snapshot of each archive dest before extraction
	// (MoveBack only). Dest folders come from FindCompressedFiles so nested
	// archive dirs are included. Kept across retries so failed cleanups are
	// not recaptured as download content. Nil means remnant handling is skipped.
	preFiles map[string]os.FileInfo
	// noRetry is set when remnant_action=off leaves a blocker.
	noRetry bool
}

type eventData struct {
	cnfg *FolderConfig
	name string
	file string
	op   string
}

func (u *Unpackerr) validateFolders() error {
	for idx := range u.Folders {
		if u.Folders[idx].DeleteAfter == nil {
			// If delete after wasn't set, then set it to 10 minutes.
			u.Folders[idx].DeleteAfter = &cnfg.Duration{Duration: defaultFolderDelete}
		}

		n, _, err := parseOptionalMaxBytes(u.Folders[idx].MaxBytes)
		if err != nil {
			return fmt.Errorf("folder %s: %w", u.Folders[idx].Path, err)
		}

		u.Folders[idx].maxBytes = n
	}

	return nil
}

func (u *Unpackerr) logFolders() {
	if epath, count := "", len(u.Folders); count == 1 {
		folder := u.Folders[0]
		if folder.ExtractPath != "" {
			epath = ", extract to: " + folder.ExtractPath
		}

		u.Printf(" => Folder Config: 1 path: %s%s; delete_after:%v delete_orig:%v delete_files:%v "+
			"log_file:%v move_back:%v isos:%v files:%d ratio:%g nested:%d extras_depth:%d symlinks:%v event_buffer:%d",
			folder.Path, epath, folder.DeleteAfter, folder.DeleteOrig, folder.DeleteFiles,
			!folder.DisableLog, folder.MoveBack, folder.ExtractISOs, folder.MaxFiles, folder.MaxRatio,
			folder.MaxNested, folder.ExtrasMaxDepth, folder.AllowSymlinks, u.Folder.Buffer)
	} else {
		u.Printf(" => Folder Config: %d paths, event_buffer:%d ", count, u.Folder.Buffer)

		for _, folder := range u.Folders {
			if epath = ""; folder.ExtractPath != "" {
				epath = " extract to: " + folder.ExtractPath
			}

			u.Printf(" =>    Path: %s%s; delete_after:%v delete_orig:%v delete_files:%v log_file:%v "+
				"move_back:%v isos:%v files:%d ratio:%g nested:%d extras_depth:%d symlinks:%v",
				folder.Path, epath, folder.DeleteAfter, folder.DeleteOrig, folder.DeleteFiles,
				!folder.DisableLog, folder.MoveBack, folder.ExtractISOs, folder.MaxFiles, folder.MaxRatio,
				folder.MaxNested, folder.ExtrasMaxDepth, folder.AllowSymlinks)
		}
	}
}

// PollFolders begins the routines to watch folders for changes.
// if those changes include the addition of compressed files, they
// are processed for exctraction.
func (u *Unpackerr) PollFolders() {
	var (
		flist []string
		err   error
	)

	if isRunningInDocker() && u.Folder.Interval.Duration == 0 {
		u.Folder.Interval.Duration = defaultPollInterval
	}

	u.Folders, flist = checkFolders(u.Folders, u.Logger)

	u.folders, err = u.Folder.newWatcher(u.Folders, u.Logger)
	if err != nil {
		u.Errorf("Watching Folders: %s", err)
		return
	}
	// do not close either watcher.

	if len(u.Folders) == 0 {
		return
	}

	go u.folders.watchFSNotify()

	u.Printf("[Folder] Watching (fsnotify): %s", strings.Join(flist, ", "))

	// Setting an interval of any value less than 5 milliseconds
	// (except zero in docker) allows disabling the poller.
	if u.Folder.Interval.Duration < minimumPollInterval {
		return
	}

	go func() {
		if err := u.folders.Watcher.Start(u.Folder.Interval.Duration); err != nil {
			u.Errorf("Folder poller stopped: %v", err)
		}
	}()

	u.Printf("[Folder] Polling @ %s: %s", u.Folder.Interval.String(), strings.Join(flist, ", "))
}

// checkFolders stats all configured folders and returns only "good" ones.
func checkFolders(folders []*FolderConfig, log Logs) ([]*FolderConfig, []string) {
	var (
		err         error
		goodFolders = folders[:0]
		goodFlist   = []string{}
	)

	for _, folder := range folders {
		folder.Path, err = filepath.Abs(expandHomedir(folder.Path))
		if err != nil {
			log.Errorf("Folder '%s' (bad path): %v", folder.Path, err)
			continue
		}

		if folder.ExtractPath != "" {
			folder.ExtractPath, err = filepath.Abs(expandHomedir(folder.ExtractPath))
			if err != nil {
				log.Errorf("Folder '%s' (bad extract path): %v", folder.ExtractPath, err)
				continue
			}
		}

		folder.ExcludePaths = normalizeFolderExcludePaths(folder.Path, folder.ExcludePaths)

		if stat, err := os.Stat(folder.Path); err != nil {
			log.Errorf("Folder '%s' (cannot watch): %v", folder.Path, err)
			continue
		} else if !stat.IsDir() {
			log.Errorf("Folder '%s' (cannot watch): not a folder", folder.Path)
			continue
		}

		goodFolders = append(goodFolders, folder)
		goodFlist = append(goodFlist, folder.Path)
	}

	return goodFolders, goodFlist
}

func normalizeFolderExcludePaths(basePath string, excludes []string) []string {
	cleaned := make([]string, 0, len(excludes))

	for _, exclude := range excludes {
		exclude = strings.TrimSpace(exclude)
		if exclude == "" {
			continue
		}

		exclude = expandHomedir(exclude)
		if !filepath.IsAbs(exclude) {
			exclude = filepath.Join(basePath, exclude)
		}

		if abs, err := filepath.Abs(exclude); err == nil {
			cleaned = append(cleaned, filepath.Clean(abs))
		}
	}

	return cleaned
}

func (c *FolderConfig) isExcludedPath(path string) bool {
	if len(c.ExcludePaths) == 0 || path == "" {
		return false
	}

	path = filepath.Clean(path)

	for _, exclude := range c.ExcludePaths {
		exclude = filepath.Clean(exclude)
		if path == exclude || strings.HasPrefix(path, exclude+string(os.PathSeparator)) {
			return true
		}
	}

	return false
}

// newWatcher returns a new folder watcher.
// You must call folders.FSNotify.Close() when you're done with it.
func (c FoldersConfig) newWatcher(folderConfig []*FolderConfig, log Logs) (*Folders, error) {
	folders := &Folders{
		Interval: c.Interval.Duration,
		Config:   folderConfig,
		Folders:  make(map[string]*Folder),
		Outputs:  make(map[string]string),
		Events:   make(chan *eventData, c.Buffer),
		Updates:  make(chan *xtractr.Response, updateChanBuf),
		Logs:     log,
	}

	if len(folderConfig) == 0 {
		return folders, nil // do not initialize watcher
	}

	folders.Watcher = watcher.New()
	folders.Watcher.FilterOps(watcher.Rename, watcher.Move, watcher.Write, watcher.Create)
	folders.Watcher.IgnoreHiddenFiles(true)

	fsn, err := fsnotify.NewWatcher()
	if err != nil {
		return folders, fmt.Errorf("fsnotify.NewWatcher: %w", err)
	}

	folders.FSNotify = fsn

	for _, folder := range folderConfig {
		if err := folders.Watcher.Add(folder.Path); err != nil {
			log.Errorf("Folder '%s' (cannot poll): %v", folder.Path, err)
		}

		if err := fsn.Add(folder.Path); err != nil {
			log.Errorf("Folder '%s' (cannot watch): %v", folder.Path, err)
		}
	}

	return folders, nil
}

// Add uses either fsnotify or watcher.
func (f *Folders) Add(folder string) error {
	if f.Interval >= minimumPollInterval {
		if err := f.Watcher.Add(folder); err != nil {
			return fmt.Errorf("watcher: %w", err)
		}

		return nil
	}

	if err := f.FSNotify.Add(folder); err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}

	return nil
}

// Remove uses either fsnotify or watcher.
func (f *Folders) Remove(folder string) {
	if f.Watcher != nil {
		_ = f.Watcher.Remove(folder)
	}

	if f.FSNotify != nil {
		_ = f.FSNotify.Remove(folder)
	}
}

// extractTrackedItem starts an archive or folder's extraction after it hasn't been written to in a while.
func (u *Unpackerr) extractTrackedItem(name string, folder *Folder, now time.Time) {
	folder.updated = now

	if u.deferIncompleteFolderExtract(name, folder, now) {
		return
	}

	exclude := folderExcludeSuffixes(name, folder.config)
	filter := xtractr.Filter{Path: name, ExcludeSuffix: exclude, AllowSymlinks: folder.config.AllowSymlinks}
	found := xtractr.FindCompressedFiles(filter)
	if found.Count() == 0 {
		// Keep the folder watched briefly so a late-arriving archive can still be seen,
		// but do not queue, notify, or show this as a completed extraction.
		folder.status = EXTRACTEDNOTHING
		u.Debugf("[Folder] Ignoring item without compressed files: %s", name)
		u.recoveryTrackFolder(name, folder.config, folder.status, now)

		return
	}

	u.folders.Remove(name) // stop the fs watcher(s).

	if outputPath := folderDerivedOutputPath(name); outputPath != "" {
		u.folders.Outputs[filepath.Clean(outputPath)] = filepath.Clean(name)
	}

	// Do not extract r00 file if rar file with same name exists.
	if strings.HasSuffix(strings.ToLower(name), ".r00") &&
		xtractr.CheckR00ForRarFile(getFileList(filepath.Dir(name)), filepath.Base(name)) {
		u.Printf("[Folder] Removing tracked item without extraction: %v (rar file exists)", name)
		u.folders.Folders[name].status = EXTRACTEDNOTHING
		u.recoveryClearFolder(name)

		return
	}

	folder.status = QUEUED

	// create a queue counter in the main history; add to u.Map and send webhook for a new folder.
	item := u.updateQueueStatus(&newStatus{Name: name, Status: QUEUED}, folder.updated, true)
	u.recoveryTrackFolder(name, folder.config, QUEUED, folder.updated)
	u.updateHistory(FolderString + ": " + name)

	u.snapshotFolderArchiveDests(name, folder, found)

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
		AllowSymlinks:    folder.config.AllowSymlinks,
		MaxBytes:         folder.config.maxBytes,
		MaxFiles:         folder.config.MaxFiles,
		MaxRatio:         folder.config.MaxRatio,
		MaxNested:        folder.config.MaxNested,
		ExtrasMaxDepth:   folder.config.ExtrasMaxDepth,
		TempFolder:       !folder.config.MoveBack,
		ExtractTo:        folder.config.ExtractPath,
		DeleteOrig:       false,
		CBChannel:        u.folders.Updates,
		CBFunction:       nil,
		Progress:         u.progressUpdateCallback(item),
		LogFile:          !folder.config.DisableLog,
		DisableRecursion: folder.config.DisableRecursion,
	})
	if err != nil {
		return 0, fmt.Errorf("queue folder extraction: %w", err)
	}

	return queueSize, nil
}

func (u *Unpackerr) snapshotFolderArchiveDests(name string, folder *Folder, found xtractr.ArchiveList) {
	if !folder.config.MoveBack {
		return
	}

	snap, err := keepDirSnapshot(folder.preFiles, archiveSnapshotPaths(name, found)...)
	if err != nil {
		u.Errorf("[Folder] Snapshot dests for remnant check: %v", err)
	} else {
		folder.preFiles = snap
	}
}

func (u *Unpackerr) deferIncompleteFolderExtract(name string, folder *Folder, now time.Time) bool {
	partial := folderIncompleteDownload(name)
	if partial == "" {
		return false
	}

	folder.status = WAITING
	u.recoveryTrackFolder(name, folder.config, folder.status, now)
	u.Debugf("[Folder] Deferring extraction while download is incomplete: %s (%s)", name, partial)

	return true
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

func (f *Folders) shouldIgnoreExtractOutput(path string, items map[string]*Extract) bool {
	if f == nil || len(f.Outputs) == 0 || path == "" {
		return false
	}

	path = filepath.Clean(path)

	source, ok := f.Outputs[path]
	if !ok {
		return false
	}

	if _, err := os.Stat(path); err != nil {
		delete(f.Outputs, path)
		return false
	}

	if _, tracked := f.Folders[source]; tracked {
		return true
	}

	if _, tracked := items[source]; tracked {
		return true
	}

	// Keep suppressing generated extraction output while it still exists on disk.
	// Otherwise a later write inside the extracted folder can be mistaken for a
	// brand-new watched item and produce a duplicate "Nothing Extracted" record.
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
func (u *Unpackerr) folderXtractrCallback(resp *xtractr.Response) {
	folder, ok := u.folders.Folders[resp.X.Name]

	switch item := u.Map[resp.X.Name]; {
	case !ok, item == nil:
		// It doesn't exist? weird. delete it and bail out.
		u.recoveryClearFolder(resp.X.Name)
		delete(u.folders.Folders, resp.X.Name)
		delete(u.Map, resp.X.Name)

		return
	case !resp.Done:
		item.XProg.Archives = resp.Archives.Count() + resp.Extras.Count()
		folder.status = EXTRACTING
		u.Printf("[Folder] Extraction Started: %s, retries: %d, items in queue: %d", resp.X.Name, folder.retries, resp.Queued)
	case errors.Is(resp.Error, xtractr.ErrNoCompressedFiles):
		folder.status = EXTRACTEDNOTHING
		u.Debugf("[Folder] Ignoring item without compressed files: %s", resp.X.Name)
		delete(u.Map, resp.X.Name)
		u.recoveryTrackFolder(resp.X.Name, folder.config, folder.status, resp.Started.Add(resp.Elapsed))

		return
	default: // this runs in a go routine
		u.finishFolderExtract(folder, resp)
	}

	folder.updated = resp.Started.Add(resp.Elapsed)
	u.recoveryTrackFolder(resp.X.Name, folder.config, folder.status, folder.updated)
	u.updateQueueStatus(&newStatus{Name: resp.X.Name, Resp: resp, Status: folder.status}, folder.updated, true)
}

// finishFolderExtract classifies refusals on any terminal response (success or
// error), then keeps EXTRACTFAILED when remnants remain or the original error
// still requires a retry. Cleared remnants restart via EXTRACTFAILED so the
// next pass is spaced by retry_delay. remnant_action=off sets noRetry so
// checkFolderStats will not restart.
func (u *Unpackerr) finishFolderExtract(folder *Folder, resp *xtractr.Response) {
	if resp.Error != nil {
		folder.archives = resp.Archives
		u.Errorf("[Folder] %s: %s: %v", EXTRACTFAILED.Desc(), resp.X.Name, resp.Error)
	} else {
		u.Printf("[Folder] Extraction Finished: %s => elapsed: %v, archives: %d, "+
			"extra archives: %d, files extracted: %d, written: %sB",
			resp.X.Name, resp.Elapsed.Round(time.Second), resp.Archives.Count(),
			resp.Extras.Count(), len(resp.NewFiles), bytefmt.ByteSize(resp.Size))
	}

	u.updateMetrics(resp, FolderString, folder.config.Path)

	if status, ok := u.handleRemnants(resp, folder.preFiles, folder.retries); ok {
		u.finishFolderRemnants(folder, resp, status)
		return
	}

	if resp.Error != nil {
		folder.status = EXTRACTFAILED
		return
	}

	folder.archives = resp.Archives
	folder.status = EXTRACTED
	folder.files = resp.NewFiles
}

func (u *Unpackerr) finishFolderRemnants(folder *Folder, resp *xtractr.Response, status ExtractStatus) {
	if status == WAITING {
		u.Printf("[Folder] Cleared interrupted-extraction remnant(s), restarting extraction: %s", resp.X.Name)

		folder.status = EXTRACTFAILED

		return
	}

	if remnantAction(u.RemnantAction) == "off" {
		folder.noRetry = true
	}

	u.Errorf("[Folder] Extraction blocked by interrupted-extraction remnant(s): %s", resp.X.Name)

	folder.status = EXTRACTFAILED
}

// watchFSNotify reads file system events from a channel and processes them.
// This runs in its own go routine, and eventually sends the event back into the main routine.
func (f *Folders) watchFSNotify() {
	defer log.Println("Folder watcher routine exited. No longer watching any folders.")

	for {
		select {
		case err := <-f.Watcher.Error:
			f.Errorf("watcher: %v", err)
		case err := <-f.FSNotify.Errors:
			f.Errorf("fsnotify: %v", err)
		case event, ok := <-f.FSNotify.Events:
			if !ok {
				return
			}

			f.handleFileEvent(event.Name, "f "+event.Op.String())
		case event := <-f.Watcher.Event:
			f.handleFileEvent(event.Path, "w "+event.Op.String())
		case <-f.Watcher.Closed:
			return
		}
	}
}

// handleFileEvent takes formatted events from fsnotify or fswatcher, does minimal
// (thread safe) validation before sending the re-formatted event to the main go routine.
func (f *Folders) handleFileEvent(name, operation string) {
	if strings.HasSuffix(name, suffix) {
		return
	}

	// Send this event to processEvent().
	for _, cnfg := range f.Config {
		// Do not handle events on the watched folder itself.
		if name == cnfg.Path {
			return
		}

		// cnfg.Path: "/Users/Documents/watched_folder"
		// event.Name: "/Users/Documents/watched_folder/new_folder/file.rar"
		// eventData.name: "new_folder"
		if !strings.HasPrefix(name, cnfg.Path) {
			continue // Not the configured folder for the event we just got.
		}

		if cnfg.isExcludedPath(name) {
			f.Debugf("Folder: Ignored event from excluded path: %v", name)
			continue
		}

		// processEvent (below) handles events sent to f.Events.
		if dir := filepath.Dir(name); dir == cnfg.Path {
			f.Events <- &eventData{name: filepath.Base(name), cnfg: cnfg, file: name, op: operation}
		} else {
			f.Events <- &eventData{name: filepath.Base(dir), cnfg: cnfg, file: name, op: operation}
		}

		return
	}

	f.Debugf("Folder: Ignored event from non-configured path: %v", name)
}

// processEvent is here to process the event in the `*Unpackerr` scope before sending it back to the `*Folders` scope.
func (u *Unpackerr) processEvent(event *eventData, now time.Time) {
	// Do not watch our own log file.
	if event.file == u.LogFile || event.file == u.Webserver.LogFile {
		return
	}

	dirPath := filepath.Join(event.cnfg.Path, event.name)
	if u.folders.shouldIgnoreExtractOutput(dirPath, u.Map) {
		u.folders.Debugf("Folder: Ignored File Event (%s) '%s' (extract output)", event.op, event.file)
		return
	}

	_, trackedBefore := u.folders.Folders[dirPath]
	u.folders.processEvent(event, now)

	if folder := u.folders.Folders[dirPath]; folder != nil {
		u.recoveryTrackFolder(dirPath, folder.config, folder.status, folder.updated)
	} else if trackedBefore {
		u.recoveryClearFolder(dirPath)
	}
}

// processEvent processes the event that was received.
func (f *Folders) processEvent(event *eventData, now time.Time) {
	dirPath := filepath.Join(event.cnfg.Path, event.name)

	if event.cnfg.isExcludedPath(event.file) || event.cnfg.isExcludedPath(dirPath) {
		f.Debugf("Folder: Ignored File Event (%s) '%s' (excluded path)", event.op, event.file)
		return
	}

	stat, err := os.Stat(dirPath)
	if err != nil {
		// Item is unusable (probably deleted), remove it from history.
		if _, ok := f.Folders[dirPath]; ok {
			f.Debugf("Folder: Removing Tracked Item: %v", dirPath)
			delete(f.Folders, dirPath)
			f.Remove(dirPath)
		}

		f.Debugf("Folder: Ignored File Event (%s) '%s' (unreadable): %v", event.op, event.file, err)

		return
	}

	if !stat.IsDir() && !xtractr.IsArchiveFile(event.name) {
		f.Debugf("Folder: Ignored File Event (%s) '%s' (not archive or dir): %v", event.op, event.file, err)
		return
	}

	f.saveEvent(event, dirPath, now)
}

func (f *Folders) saveEvent(event *eventData, dirPath string, now time.Time) {
	if existing, ok := f.Folders[dirPath]; ok {
		existing.updated = now
		if existing.status == EXTRACTEDNOTHING {
			existing.status = WAITING
		}

		return
	}

	if err := f.Add(dirPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			f.Errorf("Folder: Tracking New Item: %v (event: %s): %v ", dirPath, event.op, err)
		}

		return
	}

	f.Printf("[Folder] Tracking New Item: %v (event: %s)", dirPath, event.op)

	f.Folders[dirPath] = &Folder{
		updated: now,
		status:  WAITING,
		config:  event.cnfg,
	}
}

// checkFolderStats runs at an interval to see if any folders need work done on them.
// This runs on an interval ticker in the main go routine.
func (u *Unpackerr) checkFolderStats(now time.Time) {
	for name, folder := range u.folders.Folders {
		switch elapsed := now.Sub(folder.updated); {
		case WAITING == folder.status && elapsed >= u.StartDelay.Duration:
			// The folder hasn't been written to in a while, extract it.
			u.extractTrackedItem(name, folder, now)
		case EXTRACTEDNOTHING == folder.status:
			// Wait until this item hasn't been touched for a while, so it doesn't re-queue.
			if now.Sub(folder.updated) > u.StartDelay.Duration {
				u.folders.Remove(name)
				u.recoveryClearFolder(name)
				delete(u.Map, name)
				delete(u.folders.Folders, name)
			}
		case EXTRACTFAILED == folder.status && folder.noRetry:
			u.updateQueueStatus(&newStatus{Name: name, Status: DELETED, Resp: nil}, now, true)
			delete(u.folders.Folders, name)
			u.Printf("[Folder] Remnant left in place (remnant_action=off), giving up: %s", name)
		case EXTRACTFAILED == folder.status && elapsed >= u.RetryDelay.Duration &&
			folder.retries < u.maxRetries():
			u.Retries++
			folder.retries++
			folder.updated = now
			folder.status = WAITING
			u.recoveryTrackFolder(name, folder.config, folder.status, folder.updated)
			u.Printf("[Folder] Re-starting Failed Extraction: %s (%d/%d, failed %v ago)",
				folder.config.Path, folder.retries, u.maxRetries(), elapsed.Round(time.Second))
		case EXTRACTFAILED == folder.status && folder.retries < u.maxRetries():
			// This empty block is to avoid deleting an item that needs more retries.
		case EXTRACTFAILED == folder.status && folder.retries >= u.maxRetries():
			// Retries exhausted — clean up to prevent the item from staying in the map forever.
			u.updateQueueStatus(&newStatus{Name: name, Status: DELETED, Resp: nil}, now, true)
			delete(u.folders.Folders, name)
			u.Printf("[Folder] Retries exhausted (%d/%d), giving up: %s", folder.retries, u.maxRetries(), name)
		case EXTRACTED == folder.status && folder.config.DeleteAfter.Duration <= 0:
			// if DeleteAfter is 0 we don't delete anything. we are done.
			u.updateQueueStatus(&newStatus{Name: name, Status: DELETED, Resp: nil}, now, false)
			u.recoveryClearFolder(name)
			delete(u.folders.Folders, name)
		case EXTRACTED == folder.status && elapsed >= folder.config.DeleteAfter.Duration:
			u.deleteAfterReached(name, now, folder)
		}
	}
}

//nolint:wsl_v5
func (u *Unpackerr) deleteAfterReached(name string, now time.Time, folder *Folder) {
	var webhook bool
	// Folder reached delete delay (after extraction), nuke it.
	if folder.config.DeleteFiles && !folder.config.MoveBack {
		u.delChan <- &fileDeleteReq{Paths: []string{strings.TrimRight(name, `/\`) + suffix}}
		webhook = true
	} else if folder.config.DeleteFiles && len(folder.files) > 0 {
		u.delChan <- &fileDeleteReq{Paths: folder.files}
		webhook = true
	}

	if folder.config.DeleteOrig && !folder.config.MoveBack {
		u.delChan <- &fileDeleteReq{Paths: []string{name}}
		webhook = true
	} else if folder.config.DeleteOrig && len(folder.archives) > 0 {
		u.delChan <- &fileDeleteReq{Paths: folder.archives.List()}
		webhook = true
	}

	u.updateQueueStatus(&newStatus{Name: name, Status: DELETED, Resp: nil}, now, webhook)
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

		if sendHook {
			u.runAllHooks(u.Map[data.Name])
		}

		return u.Map[data.Name]
	}

	if data.Resp != nil {
		u.Map[data.Name].Resp = data.Resp
	}

	u.Map[data.Name].Status = data.Status
	u.Map[data.Name].Updated = now

	if sendHook {
		u.runAllHooks(u.Map[data.Name])
	}

	return u.Map[data.Name]
}
