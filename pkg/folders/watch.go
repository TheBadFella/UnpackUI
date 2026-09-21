package folders

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Unpackerr/unpackerr/pkg/extract"
	"github.com/fsnotify/fsnotify"
	"github.com/radovskyb/watcher"
	"golift.io/xtractr"
)

var rarPartPattern = regexp.MustCompile(`(?i)\.part0*([0-9]+)\.rar$`)

// NewWatcher returns a new folder watcher.
// Call Close() when you are done with it (tests). The daemon leaves it open.
func (c WatchConfig) NewWatcher(
	folderConfig []*FolderConfig,
	logger Logs,
	updateBuf int,
	ignoreSuffix string,
) (*Folders, error) {
	folders := &Folders{
		Config:       folderConfig,
		Folders:      make(map[string]*Folder),
		WatchDirs:    make(map[string]struct{}),
		Events:       make(chan *Event, c.Buffer),
		Updates:      make(chan *xtractr.Response, updateBuf),
		Logs:         logger,
		IgnoreSuffix: ignoreSuffix,
	}

	if len(folderConfig) == 0 {
		return folders, nil // do not initialize watcher
	}

	if folders.addPollers(folderConfig, logger) {
		if err := folders.openFSNotify(folderConfig, logger); err != nil {
			folders.Close()

			return folders, err
		}
	}

	return folders, nil
}

func (f *Folders) addPollers(folderConfig []*FolderConfig, logger Logs) bool {
	needFSNotify := false

	for _, folder := range folderConfig {
		if folder == nil {
			continue
		}

		if !folder.UsesPoller() {
			needFSNotify = true
			continue
		}

		poller, err := newFolderPoller(folder)
		if err != nil {
			logger.Errorf("Folder '%s' (cannot poll, using fsnotify): %v", folder.Path, err)

			needFSNotify = true

			continue
		}

		f.pollers = append(f.pollers, poller)
	}

	return needFSNotify
}

func (f *Folders) openFSNotify(folderConfig []*FolderConfig, logger Logs) error {
	fsn, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify.NewWatcher: %w", err)
	}

	f.FSNotify = fsn

	for _, folder := range folderConfig {
		if folder == nil || f.pollerFor(folder.Path) != nil {
			continue
		}

		if err := fsn.Add(folder.Path); err != nil {
			logger.Errorf("Folder '%s' (cannot watch): %v", folder.Path, err)
		}

		f.WatchDirs[filepath.Clean(folder.Path)] = struct{}{}
	}

	return nil
}

// addWatchDir registers a directory as a source of filesystem events. Watched
// directories are containers only; extraction work is stored in Folders.
func (f *Folders) addWatchDir(path string) error {
	path = filepath.Clean(path)
	if f.WatchDirs == nil {
		f.WatchDirs = make(map[string]struct{})
	}

	if _, ok := f.WatchDirs[path]; ok {
		return nil
	}

	if err := f.Add(path); err != nil {
		return err
	}

	f.WatchDirs[path] = struct{}{}

	return nil
}

// newFolderPoller watches cfg.Path non-recursively, same as fsnotify.
// Existing nested folders are not listed until Folders.Add after a new item appears.
func newFolderPoller(cfg *FolderConfig) (*folderPoller, error) {
	pollWatcher := watcher.New()
	pollWatcher.FilterOps(watcher.Rename, watcher.Move, watcher.Write, watcher.Create, watcher.Remove)
	pollWatcher.IgnoreHiddenFiles(true)

	if err := pollWatcher.Add(cfg.Path); err != nil {
		return nil, fmt.Errorf("poller: %w", err)
	}

	return &folderPoller{
		path:     cfg.Path,
		interval: cfg.Interval.Duration,
		watcher:  pollWatcher,
	}, nil
}

// Close stops pollers and fsnotify. Safe for tests; the daemon does not call this.
func (f *Folders) Close() {
	if f == nil {
		return
	}

	for _, poller := range f.pollers {
		if poller != nil && poller.watcher != nil {
			poller.watcher.Close()
		}
	}

	if f.FSNotify != nil {
		_ = f.FSNotify.Close()
	}
}

// Add watches a nested path after a new archive or folder appears.
func (f *Folders) Add(folder string) error {
	if poller := f.pollerFor(folder); poller != nil {
		if err := poller.watcher.Add(folder); err != nil {
			return fmt.Errorf("poller: %w", err)
		}

		return nil
	}

	if f.FSNotify == nil {
		return nil
	}

	if err := f.FSNotify.Add(folder); err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}

	return nil
}

// Remove drops a nested watch when extract starts or the item goes away.
func (f *Folders) Remove(folder string) {
	if poller := f.pollerFor(folder); poller != nil {
		_ = poller.watcher.Remove(folder)
	}

	if f.FSNotify != nil {
		_ = f.FSNotify.Remove(folder)
	}
}

// StartPollers starts one radovskyb watcher per polled folder.
func (f *Folders) StartPollers() {
	for _, poller := range f.pollers {
		go f.startPoller(poller)
	}
}

func (f *Folders) startPoller(poller *folderPoller) {
	if err := poller.watcher.Start(poller.interval); err != nil {
		f.Errorf("folder poller stopped: %v", err)
	}
}

// PollerSummaries lists each poller as "path @ interval" for startup logs.
func (f *Folders) PollerSummaries() []string {
	out := make([]string, 0, len(f.pollers))
	for _, poller := range f.pollers {
		out = append(out, poller.path+" @ "+poller.interval.String())
	}

	return out
}

// FSNotifyPaths are watch roots that use filesystem events, not a poller.
func (f *Folders) FSNotifyPaths() []string {
	out := make([]string, 0, len(f.Config))
	for _, cfg := range f.Config {
		if cfg == nil || f.pollerFor(cfg.Path) != nil {
			continue
		}

		out = append(out, cfg.Path)
	}

	return out
}

func (f *Folders) pollerFor(path string) *folderPoller {
	cfg := f.watchConfig(path)
	if cfg == nil {
		return nil
	}

	for _, poller := range f.pollers {
		if poller != nil && poller.path == cfg.Path {
			return poller
		}
	}

	return nil
}

func (f *Folders) watchConfig(name string) *FolderConfig {
	name = filepath.Clean(name)

	var (
		best    *FolderConfig
		bestLen = -1
	)

	for _, cfg := range f.Config {
		if cfg == nil || cfg.Path == "" {
			continue
		}

		clean := filepath.Clean(cfg.Path)
		if !PathContains(clean, name) {
			continue
		}

		if len(clean) > bestLen {
			best = cfg
			bestLen = len(clean)
		}
	}

	return best
}

// WatchFSNotify reads file system events from a channel and processes them.
// This runs in its own go routine, and eventually sends the event back into the main routine.
func (f *Folders) WatchFSNotify() {
	defer log.Println("Folder watcher routine exited. No longer watching any folders.")

	var waitGroup sync.WaitGroup

	for _, poller := range f.pollers {
		waitGroup.Add(1)

		go func(poller *folderPoller) {
			defer waitGroup.Done()

			f.watchPoller(poller)
		}(poller)
	}

	if f.FSNotify != nil {
		f.readFSNotify()
	}

	waitGroup.Wait()
}

func (f *Folders) watchPoller(poller *folderPoller) {
	for {
		select {
		case err := <-poller.watcher.Error:
			f.Errorf("watcher: %v", err)
		case event := <-poller.watcher.Event:
			f.handleFileEvent(event.Path, "w "+event.Op.String())
		case <-poller.watcher.Closed:
			return
		}
	}
}

func (f *Folders) readFSNotify() {
	for {
		select {
		case err := <-f.FSNotify.Errors:
			f.Errorf("fsnotify: %v", err)
		case event, ok := <-f.FSNotify.Events:
			if !ok {
				return
			}

			f.handleFileEvent(event.Name, "f "+event.Op.String())
		}
	}
}

func (f *Folders) handleFileEvent(name, operation string) {
	if f.ignoredExtractName(name) {
		return
	}

	cfg := f.watchConfig(name)
	if cfg == nil {
		f.Debugf("Folder: Ignored event from non-configured path: %v", name)
		return
	}

	if filepath.Clean(name) == filepath.Clean(cfg.Path) {
		return
	}

	if cfg.IsExcludedPath(name) {
		f.Debugf("Folder: Ignored event from excluded path: %v", name)
		return
	}

	// Keep the exact changed path. Directories are scan/watch containers;
	// archive files below them must be tracked as separate work items.
	f.Events <- &Event{Name: filepath.Base(name), Config: cfg, File: name, Op: operation}
}

// ProcessEvent processes the event that was received and returns every task
// path affected by it. Directory events include their discovered archive files.
func (f *Folders) ProcessEvent(event *Event, now time.Time) []string {
	dirPath, ok := eventPath(event)
	if !ok {
		f.Debugf("Folder: Ignored File Event (%s) '%s' (outside configured path)", event.Op, event.File)
		return nil
	}

	if event.Config.IsExcludedPath(event.File) || event.Config.IsExcludedPath(dirPath) {
		f.Debugf("Folder: Ignored File Event (%s) '%s' (excluded path)", event.Op, event.File)
		return []string{dirPath}
	}

	stat, err := os.Stat(dirPath)
	if err != nil {
		return f.handleUnreadableEvent(event, dirPath, err)
	}

	if !stat.IsDir() && !xtractr.IsArchiveFile(filepath.Base(dirPath)) {
		f.Debugf("Folder: Ignored File Event (%s) '%s' (not archive or dir): %v", event.Op, event.File, err)
		return []string{dirPath}
	}

	if f.ignoredExtractName(dirPath) || f.ignoredExtractName(event.File) {
		f.Debugf("Folder: Ignored File Event (%s) '%s' (extract path)", event.Op, event.File)
		return []string{dirPath}
	}

	if stat.IsDir() && f.isExtractDest(dirPath) {
		f.Debugf("Folder: Ignored File Event (%s) '%s' (extract output)", event.Op, event.File)

		f.Debugf("Folder: Removing Tracked Item: %v", dirPath)
		delete(f.Folders, dirPath)

		return []string{dirPath}
	}

	if stat.IsDir() {
		// A directory is a scan/watch container. Scan it now so a moved or
		// quickly copied tree cannot outrun watcher registration, then watch its
		// descendants for later archive writes.
		discovered := f.scanWatchDir(event, dirPath, now, true)
		if len(discovered) > 0 {
			delete(f.Folders, dirPath)
			return append([]string{dirPath}, discovered...)
		}

		if event.Config != nil && ((len(event.Config.WaitExtensions) > 0 &&
			WaitFileInTop(dirPath, event.Config.WaitExtensions) != "") || event.Config.SkipEmpty) {
			f.saveEvent(event, dirPath, now)
			return []string{dirPath}
		}

		delete(f.Folders, dirPath)
		return []string{dirPath}
	}

	if f.insideExtractDest(event.Config.Path, filepath.Dir(dirPath)) || !isPrimaryArchive(dirPath) {
		f.Debugf("Folder: Ignored File Event (%s) '%s' (extract output or secondary volume)", event.Op, event.File)
		return []string{dirPath}
	}

	f.saveEvent(event, dirPath, now)

	return []string{dirPath}
}

// handleUnreadableEvent drops a tracked item whose path can no longer be stat'd
// (probably deleted) and reports the ignored event.
func (f *Folders) handleUnreadableEvent(event *Event, dirPath string, err error) []string {
	f.removeWatchDirs(dirPath)

	// Item is unusable (probably deleted), remove it from history.
	if _, ok := f.Folders[dirPath]; ok {
		f.Debugf("Folder: Removing Tracked Item: %v", dirPath)
		delete(f.Folders, dirPath)
		f.Remove(dirPath)
	}

	f.Debugf("Folder: Ignored File Event (%s) '%s' (unreadable): %v", event.Op, event.File, err)

	return []string{dirPath}
}

// Scan discovers archives that already exist when Unpackerr starts. Existing
// recovered tasks are returned but not refreshed, preserving their timestamps.
func (f *Folders) Scan(now time.Time) []string {
	paths := make([]string, 0, len(f.Config))
	for _, cfg := range f.Config {
		event := &Event{Config: cfg, Name: filepath.Base(cfg.Path), File: cfg.Path, Op: "startup scan"}
		paths = append(paths, f.scanWatchDir(event, filepath.Clean(cfg.Path), now, false)...)
	}

	return paths
}

func (f *Folders) removeWatchDirs(path string) {
	for watched := range f.WatchDirs {
		if !isWithinPath(path, watched) {
			continue
		}

		f.Remove(watched)
		delete(f.WatchDirs, watched)
	}
}

func eventPath(event *Event) (string, bool) {
	if event == nil || event.Config == nil {
		return "", false
	}

	path := event.File
	if path == "" || hasWaitSuffix(path, event.Config.WaitExtensions) {
		path = filepath.Join(event.Config.Path, event.Name)
	}
	path = filepath.Clean(path)

	return path, isWithinPath(event.Config.Path, path) && filepath.Clean(path) != filepath.Clean(event.Config.Path)
}

func isWithinPath(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}

	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func (f *Folders) scanWatchDir(event *Event, dirPath string, now time.Time, refreshExisting bool) []string {
	paths := []string{}
	if err := f.addWatchDir(dirPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		f.Errorf("Folder: Watching directory %v (event: %s): %v", dirPath, event.Op, err)
	}

	err := filepath.WalkDir(dirPath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		cfg := event.Config
		if matched := f.watchConfig(path); matched != nil {
			cfg = matched
		}

		if cfg.IsExcludedPath(path) || f.ignoredExtractName(path) {
			if entry.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}

		if entry.IsDir() {
			if path != dirPath && f.isExtractDest(path) {
				return filepath.SkipDir
			}

			if err := f.addWatchDir(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				f.Errorf("Folder: Watching directory %v (event: %s): %v", path, event.Op, err)
			}

			return nil
		}

		if !xtractr.IsArchiveFile(entry.Name()) {
			return nil
		}
		if f.insideExtractDest(cfg.Path, filepath.Dir(path)) || !isPrimaryArchive(path) {
			return nil
		}

		f.saveEvent(&Event{Config: cfg, Name: entry.Name(), File: path, Op: event.Op}, path, now, refreshExisting)
		paths = append(paths, path)

		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		f.Errorf("Folder: Scanning directory %v (event: %s): %v", dirPath, event.Op, err)
	}

	return paths
}

func (f *Folders) saveEvent(event *Event, dirPath string, now time.Time, refreshExisting ...bool) {
	if _, ok := f.Folders[dirPath]; ok {
		if len(refreshExisting) == 0 || refreshExisting[0] {
			f.Folders[dirPath].Updated = now
		}
		return
	}

	if err := f.Add(dirPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			f.Errorf("Folder: Tracking New Item: %v (event: %s): %v ", dirPath, event.Op, err)
		}

		return
	}

	f.Printf("[Folder] Tracking New Item: %v (event: %s)", dirPath, event.Op)

	f.Folders[dirPath] = &Folder{
		Updated: now,
		Status:  extract.WAITING,
		Config:  event.Config,
	}
}

// insideExtractDest prevents archive-shaped extraction output from becoming
// new watch work. The watch root itself is never treated as output.
func (f *Folders) insideExtractDest(root, dir string) bool {
	root = filepath.Clean(root)
	for dir = filepath.Clean(dir); isWithinPath(root, dir) && dir != root; dir = filepath.Dir(dir) {
		if f.isExtractDest(dir) {
			return true
		}
	}

	return false
}

// isPrimaryArchive keeps one task per multipart set. part01.rar (or part1.rar)
// is the extractable entry point; later volumes are data for that task.
func isPrimaryArchive(path string) bool {
	match := rarPartPattern.FindStringSubmatch(filepath.Base(path))
	if len(match) != rarPartPattern.NumSubexp()+1 {
		return true
	}

	part, err := strconv.Atoi(match[1])
	return err != nil || part == 1
}

// ignoredExtractName is true when a path component is the temp extract folder
// (ends with IgnoreSuffix) or the extract log (_unpackerred.<archive>.txt).
func (f *Folders) ignoredExtractName(path string) bool {
	if f == nil || f.IgnoreSuffix == "" || path == "" {
		return false
	}

	for {
		base := filepath.Base(path)
		if strings.HasSuffix(base, f.IgnoreSuffix) || strings.HasPrefix(base, f.IgnoreSuffix+".") {
			return true
		}

		next := filepath.Dir(path)
		if next == path {
			return false
		}

		path = next
	}
}

// isExtractDest reports whether dir is xtractr output: it has the extract log,
// or it sits next to an archive whose stem matches the directory name
// (movie.iso → movie/ after the temp _unpackerred folder is renamed).
func (f *Folders) isExtractDest(dirPath string) bool {
	if hasExtractLog(dirPath, f.IgnoreSuffix) {
		return true
	}

	return hasSiblingArchiveStem(dirPath)
}

func hasExtractLog(dirPath, suffix string) bool {
	if suffix == "" {
		return false
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false
	}

	prefix := suffix + "."

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(strings.ToLower(name), ".txt") {
			return true
		}
	}

	return false
}

func hasSiblingArchiveStem(dirPath string) bool {
	base := filepath.Base(dirPath)
	parent := filepath.Dir(dirPath)

	entries, err := os.ReadDir(parent)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if xtractr.IsArchiveFile(name) && archiveStem(name) == base {
			return true
		}
	}

	return false
}

// archiveStem strips archive extensions the same way xtractr names the final
// extract folder (twice, for tar.gz and friends).
func archiveStem(name string) string {
	stem := name

	for range 2 {
		if !xtractr.IsArchiveFile(stem) {
			break
		}

		next := strings.TrimSuffix(stem, filepath.Ext(stem))
		if next == stem {
			break
		}

		stem = next
	}

	return stem
}
