package unpackerr

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	recoveryStateVersion = 1
	defaultStateFile     = "unpackerr.state.json"
)

type recoveryState struct {
	Version int                        `json:"version"`
	Updated time.Time                  `json:"updated"`
	Folders map[string]*recoveryFolder `json:"folders,omitempty"`
}

type recoveryFolder struct {
	Path      string    `json:"path"`
	WatchPath string    `json:"watchPath"`
	Status    string    `json:"status"`
	Updated   time.Time `json:"updated"`
}

func newRecoveryState() *recoveryState {
	return &recoveryState{
		Version: recoveryStateVersion,
		Folders: make(map[string]*recoveryFolder),
	}
}

func (u *Unpackerr) setupRecoveryState() {
	u.StateFile = u.defaultRecoveryStatePath()
	if u.StateFile == "" {
		u.recovery = newRecoveryState()
		return
	}

	if err := os.MkdirAll(filepath.Dir(u.StateFile), logsDirMode); err != nil {
		u.Errorf("Recovery state disabled, cannot create state file directory %s: %v", filepath.Dir(u.StateFile), err)
		u.StateFile = ""
		u.recovery = newRecoveryState()
		return
	}

	state, err := readRecoveryState(u.StateFile)
	if err != nil {
		u.Errorf("Recovery state unavailable, starting with empty state: %v", err)
		u.recovery = newRecoveryState()
		return
	}

	u.recovery = state
}

func (u *Unpackerr) defaultRecoveryStatePath() string {
	switch {
	case strings.EqualFold(strings.TrimSpace(u.StateFile), "off"):
		return ""
	case u.StateFile != "":
		return normalizeRecoveryStatePath(u.StateFile)
	case isRunningInDocker():
		if stat, err := os.Stat("/config"); err == nil && stat.IsDir() {
			return filepath.Join("/config", defaultStateFile)
		}
	}

	if u.LogFile != "" {
		return filepath.Join(filepath.Dir(u.LogFile), defaultStateFile)
	}

	return normalizeRecoveryStatePath(filepath.Join("~", ".unpackerr", defaultStateFile))
}

func normalizeRecoveryStatePath(path string) string {
	path = expandHomedir(path)
	if stat, err := os.Stat(path); err == nil && stat.IsDir() {
		return filepath.Join(path, defaultStateFile)
	}

	return path
}

func readRecoveryState(path string) (*recoveryState, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return newRecoveryState(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read recovery state: %w", err)
	}

	state := newRecoveryState()
	if err := json.Unmarshal(content, state); err != nil {
		return nil, fmt.Errorf("parse recovery state %s: %w", path, err)
	}
	if state.Version == 0 {
		state.Version = recoveryStateVersion
	}
	if state.Folders == nil {
		state.Folders = make(map[string]*recoveryFolder)
	}

	return state, nil
}

func (u *Unpackerr) saveRecoveryState() {
	if u == nil || u.recovery == nil || u.StateFile == "" {
		return
	}

	if len(u.recovery.Folders) == 0 {
		if err := os.Remove(u.StateFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			u.Errorf("Removing recovery state file: %v", err)
		}

		return
	}

	u.recovery.Version = recoveryStateVersion
	u.recovery.Updated = time.Now()

	content, err := json.MarshalIndent(u.recovery, "", "  ")
	if err != nil {
		u.Errorf("Encoding recovery state: %v", err)
		return
	}

	if err := os.MkdirAll(filepath.Dir(u.StateFile), logsDirMode); err != nil {
		u.Errorf("Creating recovery state directory: %v", err)
		return
	}

	tmp := u.StateFile + ".tmp"
	if err := os.WriteFile(tmp, content, defaultLogFileMode); err != nil {
		u.Errorf("Writing recovery state file: %v", err)
		return
	}

	if err := os.Rename(tmp, u.StateFile); err != nil {
		if removeErr := os.Remove(u.StateFile); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			u.Errorf("Replacing recovery state file: %v", err)
			return
		}

		if err := os.Rename(tmp, u.StateFile); err != nil {
			u.Errorf("Replacing recovery state file: %v", err)
		}
	}
}

func (u *Unpackerr) recoveryTrackFolder(path string, cfg *FolderConfig, status ExtractStatus, updated time.Time) {
	if u == nil || u.recovery == nil || u.StateFile == "" || path == "" || cfg == nil {
		return
	}

	if status > EXTRACTING {
		u.recoveryClearFolder(path)
		return
	}

	path = filepath.Clean(path)
	watchPath := filepath.Clean(cfg.Path)
	statusText := status.String()
	if existing := u.recovery.Folders[path]; existing != nil &&
		existing.WatchPath == watchPath &&
		existing.Status == statusText &&
		existing.Updated.Equal(updated) {
		return
	}

	u.recovery.Folders[path] = &recoveryFolder{
		Path:      path,
		WatchPath: watchPath,
		Status:    statusText,
		Updated:   updated,
	}

	u.saveRecoveryState()
}

func (u *Unpackerr) recoveryClearFolder(path string) {
	if u == nil || u.recovery == nil || u.StateFile == "" || path == "" {
		return
	}

	delete(u.recovery.Folders, filepath.Clean(path))
	u.saveRecoveryState()
}

func (u *Unpackerr) recoverInterruptedFolders(now time.Time) {
	if u == nil || u.recovery == nil || u.folders == nil || len(u.recovery.Folders) == 0 {
		return
	}

	var recovered, skipped int

	for path, item := range u.recovery.Folders {
		if item == nil {
			delete(u.recovery.Folders, path)
			continue
		}

		item.Path = filepath.Clean(item.Path)
		if item.Path != path {
			delete(u.recovery.Folders, path)
			u.recovery.Folders[item.Path] = item
		}

		cfg := u.recoveryFolderConfig(item.Path, item.WatchPath)
		if cfg == nil {
			u.Printf("[Folder] Removing stale recovery item outside configured watch folders: %s", item.Path)
			delete(u.recovery.Folders, item.Path)
			skipped++
			continue
		}

		if _, err := os.Stat(item.Path); err != nil {
			u.Printf("[Folder] Removing stale recovery item, path no longer exists: %s (%v)", item.Path, err)
			delete(u.recovery.Folders, item.Path)
			skipped++
			continue
		}

		if _, ok := u.folders.Folders[item.Path]; ok {
			continue
		}

		updated := item.Updated
		if updated.IsZero() || item.Status == QUEUED.String() || item.Status == EXTRACTING.String() {
			updated = now.Add(-u.StartDelay.Duration)
		}

		u.folders.Folders[item.Path] = &Folder{
			updated: updated,
			status:  WAITING,
			config:  cfg,
		}

		if outputPath := folderDerivedOutputPath(item.Path); outputPath != "" {
			u.folders.Outputs[filepath.Clean(outputPath)] = item.Path
		}

		item.Status = WAITING.String()
		item.Updated = now
		item.WatchPath = filepath.Clean(cfg.Path)
		recovered++
		u.Printf("[Folder] Recovered interrupted extraction: %s", item.Path)
	}

	if recovered > 0 || skipped > 0 {
		u.saveRecoveryState()
	}
}

func (u *Unpackerr) recoveryFolderConfig(path, watchPath string) *FolderConfig {
	path = filepath.Clean(path)
	watchPath = filepath.Clean(watchPath)

	for _, cfg := range u.Folders {
		if cfg == nil || cfg.Path == "" {
			continue
		}

		cfgPath := filepath.Clean(cfg.Path)
		if watchPath != "." && cfgPath != watchPath {
			continue
		}

		if pathWithin(path, cfgPath) && !cfg.isExcludedPath(path) {
			return cfg
		}
	}

	if watchPath != "." {
		return nil
	}

	for _, cfg := range u.Folders {
		if cfg == nil || cfg.Path == "" {
			continue
		}

		cfgPath := filepath.Clean(cfg.Path)
		if pathWithin(path, cfgPath) && !cfg.isExcludedPath(path) {
			return cfg
		}
	}

	return nil
}

func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}

	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
