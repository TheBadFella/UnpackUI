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
	"time"

	"github.com/Unpackerr/unpackerr/pkg/extract"
	"github.com/fsnotify/fsnotify"
	"github.com/radovskyb/watcher"
	"golift.io/xtractr"
)

var rarPartPattern = regexp.MustCompile(`(?i)\.part0*([0-9]+)\.rar$`)

// NewWatcher returns a new folder watcher.
// You must call folders.FSNotify.Close() when you're done with it.
func (c WatchConfig) NewWatcher(
	folderConfig []*FolderConfig,
	logger Logs,
	updateBuf int,
	ignoreSuffix string,
) (*Folders, error) {
	folders := &Folders{
		Interval:     c.Interval.Duration,
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

	folders.Watcher = watcher.New()
	folders.Watcher.FilterOps(watcher.Rename, watcher.Move, watcher.Write, watcher.Create, watcher.Remove)
	folders.Watcher.IgnoreHiddenFiles(true)

	fsn, err := fsnotify.NewWatcher()
	if err != nil {
		return folders, fmt.Errorf("fsnotify.NewWatcher: %w", err)
	}

	folders.FSNotify = fsn

	for _, folder := range folderConfig {
		if err := folders.Watcher.Add(folder.Path); err != nil {
			logger.Errorf("Folder '%s' (cannot poll): %v", folder.Path, err)
		}

		if err := fsn.Add(folder.Path); err != nil {
			logger.Errorf("Folder '%s' (cannot watch): %v", folder.Path, err)
		}

		folders.WatchDirs[filepath.Clean(folder.Path)] = struct{}{}
	}

	return folders, nil
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

// Add uses either fsnotify or watcher.
func (f *Folders) Add(folder string) error {
	if f.Interval >= MinimumPollInterval {
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

// StartPoller starts the radovskyb poll watcher.
func (f *Folders) StartPoller(interval time.Duration) error {
	if err := f.Watcher.Start(interval); err != nil {
		return fmt.Errorf("folder poller stopped: %w", err)
	}

	return nil
}

// WatchFSNotify reads file system events from a channel and processes them.
// This runs in its own go routine, and eventually sends the event back into the main routine.
func (f *Folders) WatchFSNotify() {
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

func (f *Folders) handleFileEvent(name, operation string) {
	if f.ignoredExtractName(name) {
		return
	}

	for _, cfg := range f.Config {
		// Do not handle events on the watched folder itself.
		if filepath.Clean(name) == filepath.Clean(cfg.Path) {
			return
		}

		if !isWithinPath(cfg.Path, name) {
			continue // Not the configured folder for the event we just got.
		}

		if cfg.IsExcludedPath(name) {
			f.Debugf("Folder: Ignored event from excluded path: %v", name)
			continue
		}

		// Keep the exact changed path. Directories are scan/watch containers;
		// archive files below them must be tracked as separate work items.
		f.Events <- &Event{Name: filepath.Base(name), Config: cfg, File: name, Op: operation}

		return
	}

	f.Debugf("Folder: Ignored event from non-configured path: %v", name)
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
		// A directory is only a discovery boundary. Scan it now so a moved or
		// quickly copied tree cannot outrun watcher registration, then watch its
		// descendants for later archive writes.
		delete(f.Folders, dirPath)

		return append([]string{dirPath}, f.scanWatchDir(event, dirPath, now, true)...)
	}

	if f.insideExtractDest(event.Config.Path, filepath.Dir(dirPath)) || !isPrimaryArchive(dirPath) {
		f.Debugf("Folder: Ignored File Event (%s) '%s' (extract output or secondary volume)", event.Op, event.File)
		return []string{dirPath}
	}

	f.saveEvent(event, dirPath, now)

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
	if path == "" {
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

		if event.Config.IsExcludedPath(path) || f.ignoredExtractName(path) {
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
		if f.insideExtractDest(event.Config.Path, filepath.Dir(path)) || !isPrimaryArchive(path) {
			return nil
		}

		f.saveEvent(&Event{Config: event.Config, Name: entry.Name(), File: path, Op: event.Op}, path, now, refreshExisting)
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
