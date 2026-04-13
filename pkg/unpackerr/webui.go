package unpackerr

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/hako/durafmt"
	"github.com/julienschmidt/httprouter"
	"golift.io/version"
)

type webStatusSnapshot struct {
	GeneratedAt    string              `json:"generatedAt"`
	Uptime         string              `json:"uptime"`
	Stats          *Stats              `json:"stats"`
	Buffers        webStatusBuffers    `json:"buffers"`
	Counters       webStatusCounters   `json:"counters"`
	ActiveCount    int                 `json:"activeCount"`
	CompletedCount int                 `json:"completedCount"`
	Items          []webStatusItem     `json:"items"`
	dismissed      map[string]struct{} `json:"-"`
}

type webStatusBuffers struct {
	Deletes       int `json:"deletes"`
	FolderEvents  int `json:"folderEvents"`
	FolderUpdates int `json:"folderUpdates"`
	Hooks         int `json:"hooks"`
	XtractUpdates int `json:"xtractUpdates"`
}

type webStatusCounters struct {
	CmdFail  uint `json:"cmdFail"`
	CmdOK    uint `json:"cmdOK"`
	Finished uint `json:"finished"`
	HookFail uint `json:"hookFail"`
	HookOK   uint `json:"hookOK"`
	Retries  uint `json:"retries"`
}

type webStatusItem struct {
	ID          string             `json:"id"`
	Active      bool               `json:"active"`
	Completed   bool               `json:"completed"`
	App         string             `json:"app"`
	CurrentFile string             `json:"currentFile,omitempty"`
	Elapsed     string             `json:"elapsed"`
	Details     *webStatusDetails  `json:"details,omitempty"`
	Error       string             `json:"error,omitempty"`
	Name        string             `json:"name"`
	Path        string             `json:"path"`
	Progress    *webStatusProgress `json:"progress,omitempty"`
	Reason      string             `json:"reason,omitempty"`
	Status      string             `json:"status"`
	StatusText  string             `json:"statusText"`
	UpdatedAt   string             `json:"updatedAt"`
}

type webStatusDetails struct {
	Title     string            `json:"title,omitempty"`
	Bytes     string            `json:"bytes,omitempty"`
	Elapsed   string            `json:"elapsed,omitempty"`
	Output    string            `json:"output,omitempty"`
	StartedAt string            `json:"startedAt,omitempty"`
	Queue     int               `json:"queue,omitempty"`
	Archives  []string          `json:"archives,omitempty"`
	Files     []string          `json:"files,omitempty"`
	IDs       map[string]string `json:"ids,omitempty"`
}

type webStatusProgress struct {
	Archive      string  `json:"archive"`
	ArchiveCount int     `json:"archiveCount"`
	ArchiveIndex int     `json:"archiveIndex"`
	Percent      float64 `json:"percent"`
	Summary      string  `json:"summary"`
	TotalBytes   string  `json:"totalBytes"`
	WrittenBytes string  `json:"writtenBytes"`
}

func (u *Unpackerr) refreshWebState(now time.Time) {
	u.webState.Store(u.buildWebState(now))
}

func (u *Unpackerr) buildWebState(now time.Time) *webStatusSnapshot {
	prev := u.webState.Load()
	stats := u.currentStats()
	items := make([]webStatusItem, 0, len(u.Map))
	currentNames := make(map[string]struct{}, len(u.Map))
	dismissed := webStatusDismissedItems(prev)

	for name, item := range u.Map {
		currentNames[name] = struct{}{}

		webItem := buildWebStatusItem(name, item, now)
		if webItem.Completed {
			if _, skip := dismissed[webItem.ID]; skip {
				continue
			}
		}

		items = append(items, webItem)
	}

	if u.folders != nil {
		for name, folder := range u.folders.Folders {
			if _, ok := u.Map[name]; ok || folder == nil {
				continue
			}

			currentNames[name] = struct{}{}
			items = append(items, buildWaitingFolderStatusItem(name, folder, now))
		}
	}

	if prev != nil {
		for _, item := range prev.Items {
			if !item.Completed {
				continue
			}
			if _, ok := currentNames[item.Name]; ok {
				continue
			}
			if _, skip := dismissed[item.ID]; skip {
				continue
			}

			item.Elapsed = webStatusElapsed(item.UpdatedAt, now)
			items = append(items, item)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		left, right := items[i], items[j]
		if left.Completed != right.Completed {
			return !left.Completed
		}
		if a, b := webStatusRank(left.Status), webStatusRank(right.Status); a != b {
			return a < b
		}

		return left.UpdatedAt > right.UpdatedAt
	})

	buffers := webStatusBuffers{}
	if u.folders != nil {
		buffers.FolderEvents = len(u.folders.Events)
		buffers.FolderUpdates = len(u.folders.Updates)
	}

	buffers.Deletes = len(u.delChan)
	buffers.Hooks = len(u.hookChan)
	buffers.XtractUpdates = len(u.updates)

	var activeCount, completedCount int
	for _, item := range items {
		if item.Completed {
			completedCount++
		} else {
			activeCount++
		}
	}

	return &webStatusSnapshot{
		GeneratedAt:    now.Format(time.RFC3339),
		Uptime:         durafmt.Parse(now.Sub(version.Started)).LimitFirstN(3).Format(durafmtUnits),
		Stats:          stats,
		Buffers:        buffers,
		ActiveCount:    activeCount,
		CompletedCount: completedCount,
		Counters: webStatusCounters{
			CmdFail:  stats.CmdFail,
			CmdOK:    stats.CmdOK,
			Finished: u.Finished,
			HookFail: stats.HookFail,
			HookOK:   stats.HookOK,
			Retries:  u.Retries,
		},
		Items:     items,
		dismissed: dismissed,
	}
}

func buildWebStatusItem(name string, item *Extract, now time.Time) webStatusItem {
	output := webStatusItem{
		ID:         webStatusItemID(name, item.Status, item.Updated),
		Active:     !webStatusIsCompleted(item.Status.String()),
		Completed:  webStatusIsCompleted(item.Status.String()),
		App:        string(item.App),
		Elapsed:    now.Sub(item.Updated).Round(time.Second).String(),
		Details:    buildWebStatusDetails(item),
		Name:       name,
		Path:       item.Path,
		Status:     item.Status.String(),
		StatusText: item.Status.Desc(),
		UpdatedAt:  item.Updated.Format(time.RFC3339),
	}

	if item.Resp != nil && item.Resp.Error != nil {
		output.Error = item.Resp.Error.Error()
	}

	if reason, ok := item.IDs["reason"]; ok {
		output.Reason = fmt.Sprint(reason)
	}

	if item.XProg != nil {
		output.Progress = buildWebStatusProgress(item.XProg)
		if output.Progress != nil {
			output.CurrentFile = output.Progress.Archive
		}
	}

	return output
}

func buildWaitingFolderStatusItem(name string, folder *Folder, now time.Time) webStatusItem {
	reason := ""
	if folder.config != nil && folder.config.Path != "" {
		reason = "Watching folder: " + folder.config.Path
	}

	return webStatusItem{
		ID:         webStatusItemID(name, folder.status, folder.updated),
		Active:     true,
		Completed:  false,
		App:        FolderString,
		Elapsed:    now.Sub(folder.updated).Round(time.Second).String(),
		Name:       name,
		Path:       name,
		Reason:     reason,
		Status:     folder.status.String(),
		StatusText: folder.status.Desc(),
		UpdatedAt:  folder.updated.Format(time.RFC3339),
	}
}

func buildWebStatusDetails(item *Extract) *webStatusDetails {
	details := &webStatusDetails{}
	if title, ok := item.IDs["title"]; ok {
		details.Title = fmt.Sprint(title)
	}

	if len(item.IDs) > 0 {
		details.IDs = make(map[string]string, len(item.IDs))
		for key, value := range item.IDs {
			details.IDs[key] = fmt.Sprint(value)
		}
	}

	if item.Resp != nil {
		if item.Resp.Started.Unix() > 0 {
			details.StartedAt = item.Resp.Started.Format(time.RFC3339)
		}
		if item.Resp.Output != "" {
			details.Output = item.Resp.Output
		}
		if item.Resp.Size > 0 {
			details.Bytes = bytefmt.ByteSize(item.Resp.Size) + "B"
		}
		if item.Resp.Elapsed > 0 {
			details.Elapsed = item.Resp.Elapsed.Round(time.Second).String()
		}
		details.Queue = item.Resp.Queued

		for _, archiveGroup := range item.Resp.Archives {
			details.Archives = append(details.Archives, archiveGroup...)
		}
		for _, extraGroup := range item.Resp.Extras {
			details.Archives = append(details.Archives, extraGroup...)
		}
		details.Files = append(details.Files, item.Resp.NewFiles...)
	}

	if details.Title == "" && details.Bytes == "" && details.Elapsed == "" && details.Output == "" &&
		details.StartedAt == "" && details.Queue == 0 && len(details.Archives) == 0 &&
		len(details.Files) == 0 && len(details.IDs) == 0 {
		return nil
	}

	return details
}

func buildWebStatusProgress(progress *ExtractProgress) *webStatusProgress {
	if progress == nil || progress.Progress == nil {
		return nil
	}

	var wrote, total uint64
	if progress.Total > 0 {
		wrote, total = progress.Wrote, progress.Total
	} else {
		wrote, total = progress.Read, progress.Compressed
	}

	basePath := ""
	if progress.Extract != nil {
		basePath = progress.Extract.Path
	}

	archive := ""
	if progress.XFile != nil {
		archive = strings.TrimLeft(strings.TrimPrefix(progress.XFile.FilePath, basePath), `/\`)
	}

	summary := "no progress yet"
	if progress.XFile != nil && progress.Extract != nil {
		summary = progress.String()
	} else if progress.XFile != nil {
		summary = fmt.Sprintf("on archive: %d/%d @ %sB/%sB (%.0f%%): %s",
			progress.Extracted+1, progress.Archives, bytefmt.ByteSize(wrote), bytefmt.ByteSize(total),
			progress.Percent(), archive)
	}

	return &webStatusProgress{
		Archive:      archive,
		ArchiveCount: progress.Archives,
		ArchiveIndex: progress.Extracted + 1,
		Percent:      progress.Percent(),
		Summary:      summary,
		TotalBytes:   bytefmt.ByteSize(total) + "B",
		WrittenBytes: bytefmt.ByteSize(wrote) + "B",
	}
}

func webStatusRank(status string) int {
	switch status {
	case EXTRACTING.String():
		return 0
	case QUEUED.String():
		return 1
	case WAITING.String():
		return 2
	case EXTRACTFAILED.String():
		return 3
	case EXTRACTED.String():
		return 4
	case IMPORTED.String():
		return 5
	case DELETED.String(), DELETING.String():
		return 6
	case EXTRACTEDNOTHING.String():
		return 7
	default:
		return 8
	}
}

func webStatusItemID(name string, status ExtractStatus, updated time.Time) string {
	return fmt.Sprintf("%s|%s|%s", name, status.String(), updated.Format(time.RFC3339Nano))
}

func webStatusIsCompleted(status string) bool {
	switch status {
	case EXTRACTED.String(), IMPORTED.String(), DELETING.String(), DELETED.String(), EXTRACTEDNOTHING.String():
		return true
	default:
		return false
	}
}

func webStatusElapsed(updatedAt string, now time.Time) string {
	if updatedAt == "" {
		return ""
	}

	lastUpdated, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return ""
	}

	return now.Sub(lastUpdated).Round(time.Second).String()
}

func webStatusDismissedItems(snapshot *webStatusSnapshot) map[string]struct{} {
	if snapshot == nil || len(snapshot.dismissed) == 0 {
		return make(map[string]struct{})
	}

	output := make(map[string]struct{}, len(snapshot.dismissed))
	for item := range snapshot.dismissed {
		output[item] = struct{}{}
	}

	return output
}

func (u *Unpackerr) snapshotStats() *Stats {
	if snap := u.webState.Load(); snap != nil && snap.Stats != nil {
		stats := *snap.Stats
		return &stats
	}

	return &Stats{}
}

func (u *Unpackerr) currentStats() *Stats {
	stats := &Stats{}
	stats.HookOK, stats.HookFail = u.WebhookCounts()
	stats.CmdOK, stats.CmdFail = u.CmdhookCounts()

	for name := range u.Map {
		addStatusCount(stats, u.Map[name].Status)
	}

	if u.folders != nil {
		for name, folder := range u.folders.Folders {
			if _, ok := u.Map[name]; ok || folder == nil {
				continue
			}

			addStatusCount(stats, folder.status)
		}
	}

	return stats
}

func addStatusCount(stats *Stats, status ExtractStatus) {
	switch status {
	case WAITING:
		stats.Waiting++
	case QUEUED:
		stats.Queued++
	case EXTRACTING:
		stats.Extracting++
	case DELETEFAILED, EXTRACTFAILED:
		stats.Failed++
	case EXTRACTED:
		stats.Extracted++
	case DELETED, DELETING:
		stats.Deleted++
	case IMPORTED:
		stats.Imported++
	}
}

func (u *Unpackerr) webIndex(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = statusTemplate.Execute(w, nil)
}

func (u *Unpackerr) webStatusAPI(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	snapshot := u.webState.Load()
	if snapshot == nil {
		snapshot = &webStatusSnapshot{Stats: &Stats{}}
	}

	_ = json.NewEncoder(w).Encode(snapshot)
}

func (u *Unpackerr) webClearCompletedAPI(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	snapshot := u.webState.Load()
	if snapshot == nil {
		snapshot = &webStatusSnapshot{Stats: &Stats{}, dismissed: make(map[string]struct{})}
	}

	next := *snapshot
	next.dismissed = webStatusDismissedItems(snapshot)
	next.Items = make([]webStatusItem, 0, len(snapshot.Items))
	next.ActiveCount = 0
	next.CompletedCount = 0

	for _, item := range snapshot.Items {
		if item.Completed {
			next.dismissed[item.ID] = struct{}{}
			continue
		}

		next.Items = append(next.Items, item)
		next.ActiveCount++
	}

	u.webState.Store(&next)
	_ = json.NewEncoder(w).Encode(&next)
}

var statusTemplate = template.Must(template.New("status").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Unpackerr Status</title>
  <link rel="icon" type="image/svg+xml" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 64 64'%3E%3Crect width='64' height='64' rx='18' fill='%23000'/%3E%3Cpath d='M18 21h28l-4 22H22z' fill='%23121518' stroke='%23ffb04a' stroke-width='3'/%3E%3Cpath d='M24 16h16l6 7H18z' fill='%23ff8647'/%3E%3Cpath d='M32 27v12' stroke='%237ed7ff' stroke-width='4' stroke-linecap='round'/%3E%3Cpath d='m26 34 6 6 6-6' fill='none' stroke='%237ed7ff' stroke-width='4' stroke-linecap='round' stroke-linejoin='round'/%3E%3C/svg%3E">
  <style>
    :root {
      color-scheme: dark;
      --bg: #000000;
      --panel: rgba(10, 10, 10, 0.96);
      --panel-strong: rgba(18, 18, 18, 0.98);
      --panel-soft: rgba(255, 255, 255, 0.06);
      --panel-softer: rgba(255, 255, 255, 0.03);
      --panel-accent: rgba(255, 176, 74, 0.12);
      --panel-cool: rgba(126, 215, 255, 0.1);
      --text: #f3f6fb;
      --heading: #ffffff;
      --muted: #a8b0bc;
      --accent: #ffb04a;
      --accent-strong: #ff8647;
      --accent-cool: #7ed7ff;
      --accent-violet: #b79cff;
      --good: #7de7a9;
      --warn: #ffd166;
      --bad: #ff7b8f;
      --border: rgba(255, 255, 255, 0.1);
      --border-strong: rgba(255, 255, 255, 0.18);
      --shadow: 0 28px 72px rgba(0, 0, 0, 0.55);
      --font: "Segoe UI Variable Display", "Aptos", "Segoe UI", sans-serif;
      --mono: Consolas, "SFMono-Regular", monospace;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      font-family: var(--font);
      background:
        radial-gradient(circle at 12% 0%, rgba(255, 176, 74, 0.18), transparent 24%),
        radial-gradient(circle at 88% 6%, rgba(126, 215, 255, 0.16), transparent 18%),
        radial-gradient(circle at 50% 100%, rgba(183, 156, 255, 0.08), transparent 24%),
        var(--bg);
      color: var(--text);
      transition: color 160ms ease;
    }
	    .shell {
	      max-width: 1480px;
	      margin: 0 auto;
	      padding: 42px 28px 72px;
	    }
	    .hero {
	      display: grid;
	      gap: 28px;
	      margin-bottom: 34px;
	      padding: 34px;
      border: 1px solid var(--border);
      border-radius: 32px;
      background:
        linear-gradient(180deg, rgba(20, 20, 20, 0.98), rgba(8, 8, 8, 0.98)),
        linear-gradient(135deg, var(--panel-accent), transparent 35%);
      box-shadow: var(--shadow);
      position: relative;
      overflow: hidden;
    }
    .hero::before {
      content: "";
      position: absolute;
      width: 320px;
      height: 320px;
      top: -180px;
      right: -80px;
      border-radius: 50%;
      background: radial-gradient(circle, rgba(255, 176, 74, 0.22), transparent 70%);
      pointer-events: none;
    }
    .hero::after {
      content: "";
      position: absolute;
      width: 280px;
      height: 280px;
      bottom: -180px;
      left: -60px;
      border-radius: 50%;
      background: radial-gradient(circle, rgba(126, 215, 255, 0.14), transparent 72%);
      pointer-events: none;
    }
	    .headline {
	      display: flex;
	      flex-wrap: wrap;
	      gap: 24px;
      align-items: flex-start;
      justify-content: space-between;
      position: relative;
      z-index: 1;
    }
	    .hero-copy {
	      display: grid;
	      gap: 18px;
	      max-width: 780px;
	    }
	    .headline-actions {
	      display: flex;
	      flex-wrap: wrap;
	      gap: 16px;
	      align-items: center;
	      justify-content: flex-end;
	    }
    h1 {
      margin: 0;
      font-size: clamp(2.3rem, 5vw, 3.8rem);
      line-height: 0.92;
      letter-spacing: -0.045em;
      color: var(--heading);
    }
    .subtle {
      color: var(--muted);
      font-size: 1rem;
      line-height: 1.65;
    }
	    .stamp-chip {
	      display: inline-flex;
	      align-items: center;
	      gap: 10px;
	      padding: 13px 18px;
	      min-height: 48px;
	      border-radius: 18px;
      border: 1px solid rgba(126, 215, 255, 0.18);
      background: linear-gradient(180deg, rgba(126, 215, 255, 0.1), rgba(126, 215, 255, 0.05));
      color: var(--text);
      box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
    }
    .stamp-chip::before {
      content: "";
      width: 10px;
      height: 10px;
      border-radius: 50%;
      background: var(--accent-cool);
      box-shadow: 0 0 0 6px rgba(126, 215, 255, 0.1);
      flex: 0 0 auto;
    }
	    .action-button {
      appearance: none;
      border: 1px solid var(--border-strong);
      background: linear-gradient(180deg, rgba(255, 255, 255, 0.08), rgba(255, 255, 255, 0.04));
      color: var(--text);
      border-radius: 18px;
	      padding: 12px 18px;
      font: inherit;
      font-size: 0.9rem;
      font-weight: 600;
      cursor: pointer;
      box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
      transition: transform 140ms ease, border-color 140ms ease, background 140ms ease, box-shadow 140ms ease;
    }
    .action-button:hover {
      transform: translateY(-1px);
      border-color: rgba(255, 176, 74, 0.36);
      background: linear-gradient(180deg, rgba(255, 176, 74, 0.15), rgba(255, 134, 71, 0.08));
      box-shadow: 0 0 0 1px rgba(255, 176, 74, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.05);
    }
    .action-button:disabled {
      opacity: 0.45;
      cursor: default;
      transform: none;
      box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
    }
	    .grid {
	      display: grid;
	      gap: 22px;
	      grid-template-columns: repeat(auto-fit, minmax(132px, 1fr));
	    }
    .card, .table-card {
      background:
        linear-gradient(180deg, rgba(18, 18, 18, 0.98), rgba(8, 8, 8, 0.98)),
        linear-gradient(135deg, rgba(255, 176, 74, 0.04), transparent 45%);
      border: 1px solid var(--border);
      border-radius: 28px;
      box-shadow: var(--shadow);
    }
	    .card {
	      padding: 24px;
	      position: relative;
	      overflow: hidden;
	    }
    .card::before {
      content: "";
      position: absolute;
      inset: 0 auto 0 0;
      width: 6px;
      background: var(--muted);
      opacity: 0.72;
    }
    .card .label {
      color: var(--muted);
      font-size: 0.82rem;
      text-transform: uppercase;
      letter-spacing: 0.08em;
    }
	    .card .value {
	      margin-top: 16px;
      color: var(--heading);
      font-size: 2.35rem;
      font-weight: 900;
      letter-spacing: -0.04em;
      line-height: 1;
    }
    .card.tone-extracting::before {
      background: linear-gradient(180deg, var(--accent), var(--accent-strong));
    }
    .card.tone-queued::before,
    .card.tone-waiting::before {
      background: var(--warn);
    }
    .card.tone-failed::before {
      background: var(--bad);
    }
    .card.tone-extracted::before,
    .card.tone-imported::before,
    .card.tone-deleted::before {
      background: var(--good);
    }
	    .rail {
	      display: grid;
	      gap: 16px;
	      margin-top: 6px;
	      position: relative;
	      z-index: 1;
	      grid-template-columns: minmax(0, 1fr);
	    }
	    .pill {
	      display: flex;
	      align-items: center;
	      gap: 6px;
	      flex-wrap: nowrap;
	      background: linear-gradient(180deg, var(--panel-soft), rgba(255, 255, 255, 0.03));
	      border: 1px solid var(--border);
	      color: var(--muted);
	      border-radius: 16px;
	      padding: 12px 16px;
	      font-size: 0.9rem;
	      min-width: 0;
	      white-space: nowrap;
	    }
    .pill strong {
	      color: var(--text);
	      font-weight: 700;
	      flex: 0 0 auto;
	    }
	    .table-card {
	      overflow: hidden;
	      margin-top: 26px;
	    }
	    .table-head {
	      display: flex;
	      justify-content: space-between;
	      gap: 18px;
	      align-items: center;
	      padding: 28px 28px 18px;
	    }
	    .table-wrap {
	      overflow-x: auto;
	      padding: 0 18px 18px;
	    }
    table {
      width: 100%;
      border-collapse: collapse;
      min-width: 760px;
    }
	    th, td {
	      text-align: left;
	      padding: 22px 18px;
	      border-top: 1px solid var(--border);
	      vertical-align: top;
	    }
    th {
      color: var(--muted);
      font-size: 0.82rem;
      text-transform: uppercase;
      letter-spacing: 0.08em;
      font-weight: 600;
    }
    tr:hover td {
      background: rgba(255, 255, 255, 0.028);
    }
    .items-row {
      cursor: pointer;
    }
    .items-row.is-selected td {
      background: rgba(255, 176, 74, 0.08);
    }
    .status {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      background: rgba(255, 255, 255, 0.04);
      border: 1px solid transparent;
      border-radius: 999px;
      padding: 7px 13px;
      white-space: nowrap;
      font-size: 0.86rem;
    }
    .status::before {
      content: "";
      width: 9px;
      height: 9px;
      border-radius: 50%;
      background: currentColor;
      box-shadow: 0 0 0 4px rgba(255,255,255,0.05);
    }
    .status.extracting {
      color: var(--accent);
      background: rgba(255, 176, 74, 0.16);
      border-color: rgba(255, 176, 74, 0.24);
    }
    .status.queued,
    .status.waiting {
      color: var(--warn);
      background: rgba(255, 209, 102, 0.14);
      border-color: rgba(255, 209, 102, 0.24);
    }
    .status.extractfailed,
    .status.deletefailed {
      color: var(--bad);
      background: rgba(255, 123, 143, 0.14);
      border-color: rgba(255, 123, 143, 0.24);
    }
    .status.extracted,
    .status.imported,
    .status.deleted,
    .status.deleting {
      color: var(--good);
      background: rgba(125, 231, 169, 0.14);
      border-color: rgba(125, 231, 169, 0.24);
    }
    .status.extractednothing {
      color: var(--muted);
      background: var(--panel-soft);
      border-color: var(--border);
    }
    .muted {
      color: var(--muted);
    }
    .path {
      font-family: var(--mono);
      font-size: 0.82rem;
      color: var(--text);
      word-break: break-all;
      line-height: 1.55;
    }
    .progress {
      display: grid;
      gap: 10px;
      min-width: 220px;
    }
    .progress-bar {
      position: relative;
      width: 100%;
      height: 12px;
      border-radius: 999px;
      background: rgba(255, 255, 255, 0.05);
      border: 1px solid var(--border);
      overflow: hidden;
    }
    .progress-bar > span {
      position: relative;
      display: block;
      height: 100%;
      background: linear-gradient(90deg, var(--accent), var(--accent-strong) 52%, var(--accent-cool));
      border-radius: inherit;
      transition: width 360ms cubic-bezier(0.22, 1, 0.36, 1);
    }
    .progress-bar > span::after {
      content: "";
      position: absolute;
      inset: 0;
      background: linear-gradient(90deg, transparent, rgba(255,255,255,0.28), transparent);
      transform: translateX(-120%);
      animation: shimmer 2.2s linear infinite;
    }
	    .empty {
	      padding: 48px 22px 54px;
	      text-align: center;
	      color: var(--muted);
	    }
    .detail-card {
      padding-bottom: 18px;
    }
	    .detail-grid {
	      display: grid;
	      gap: 18px;
	      grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
	      padding: 12px;
	    }
	    .detail-field {
	      border: 1px solid var(--border);
	      border-radius: 22px;
	      padding: 18px;
	      background: linear-gradient(180deg, var(--panel-soft), rgba(255, 255, 255, 0.03));
	    }
    .detail-field .label {
      color: var(--muted);
      font-size: 0.78rem;
      letter-spacing: 0.08em;
      text-transform: uppercase;
      margin-bottom: 8px;
    }
    .detail-field .value {
      color: var(--text);
      line-height: 1.55;
      word-break: break-word;
    }
	    .detail-lists {
	      display: grid;
	      gap: 18px;
	      padding: 12px;
	    }
	    .detail-list {
	      border: 1px solid var(--border);
	      border-radius: 22px;
	      background: linear-gradient(180deg, var(--panel-soft), rgba(255, 255, 255, 0.03));
	      padding: 18px;
	    }
    .detail-list strong {
      display: block;
      margin-bottom: 10px;
      color: var(--heading);
    }
    .detail-list ul {
      margin: 0;
      padding-left: 18px;
    }
    .detail-list li {
      margin: 0 0 8px;
      line-height: 1.45;
      word-break: break-word;
    }
    .detail-list code {
      font-family: var(--mono);
      font-size: 0.82rem;
    }
    @keyframes shimmer {
      from {
        transform: translateX(-120%);
      }
      to {
        transform: translateX(120%);
      }
    }
	    @media (max-width: 720px) {
	      .shell {
	        padding: 24px 16px 48px;
	      }
      .headline-actions {
        justify-content: flex-start;
      }
      .table-head {
        display: grid;
      }
	      .hero {
	        padding: 22px;
	      }
	    }
	    @media (min-width: 760px) {
	      .rail {
	        grid-template-columns: repeat(2, minmax(0, 1fr));
	      }
	    }
	    @media (min-width: 1120px) {
	      .grid {
	        grid-template-columns: repeat(7, minmax(0, 1fr));
	      }
	      .rail {
	        grid-template-columns: repeat(3, minmax(0, 1fr));
	      }
	    }
	  </style>
</head>
<body>
  <div class="shell">
    <section class="hero">
      <div class="headline">
        <div class="hero-copy">
          <h1>Unpackerr Status</h1>
          <div class="subtle">Live extraction progress.</div>
        </div>
        <div class="headline-actions">
          <div class="stamp-chip" id="stamp">Loading...</div>
        </div>
      </div>
      <div class="grid" id="stat-grid"></div>
      <div class="rail" id="meta-rail"></div>
    </section>
	    <section class="table-card">
	      <div class="table-head">
	        <div>
	          <strong>Tracked Items</strong>
	          <div class="subtle">Current queue plus completed items that stay visible until you clear them.</div>
	        </div>
	        <div class="headline-actions">
	          <button class="action-button" id="clear-completed" type="button">Clear completed</button>
	          <div class="subtle" id="item-count"></div>
	        </div>
	      </div>
	      <div class="table-wrap">
	        <table>
          <thead>
            <tr>
              <th>Item</th>
              <th>Status</th>
              <th>Progress</th>
              <th>Updated</th>
              <th>Path</th>
            </tr>
          </thead>
          <tbody id="items">
            <tr><td colspan="5" class="empty">Waiting for status data...</td></tr>
          </tbody>
	        </table>
	      </div>
	    </section>
	    <section class="table-card detail-card" id="detail-card" hidden>
	      <div class="table-head">
	        <div>
	          <strong>Task Details</strong>
	          <div class="subtle" id="detail-subtitle">Click a task row to inspect its details.</div>
	        </div>
	        <div class="headline-actions">
	          <button class="action-button" id="detail-close" type="button">Close</button>
	        </div>
	      </div>
	      <div class="table-wrap">
	        <div class="detail-grid" id="detail-grid"></div>
	        <div class="detail-lists" id="detail-lists"></div>
	      </div>
	    </section>
	  </div>
	  <script>
	    const statOrder = [
	      ['extracting', 'Extracting'],
	      ['queued', 'Queued'],
	      ['waiting', 'Waiting'],
	      ['failed', 'Failed'],
	      ['extracted', 'Extracted'],
	      ['imported', 'Imported'],
	      ['deleted', 'Deleted']
	    ];

	    const statGrid = document.getElementById('stat-grid');
	    const metaRail = document.getElementById('meta-rail');
	    const stamp = document.getElementById('stamp');
	    const clearCompletedButton = document.getElementById('clear-completed');
	    const itemCount = document.getElementById('item-count');
	    const itemsBody = document.getElementById('items');
	    const detailCard = document.getElementById('detail-card');
	    const detailSubtitle = document.getElementById('detail-subtitle');
	    const detailGrid = document.getElementById('detail-grid');
	    const detailLists = document.getElementById('detail-lists');
	    const detailClose = document.getElementById('detail-close');
	    const statusUrl = new URL('api/status', window.location.href);
	    const clearCompletedUrl = new URL('api/status/clear-completed', window.location.href);
	    let lastSnapshot = { items: [] };
	    let selectedItemId = '';

	    function escapeHtml(value) {
	      return String(value ?? '').replace(/[&<>"']/g, (char) => ({
	        '&': '&amp;',
	        '<': '&lt;',
	        '>': '&gt;',
	        '"': '&quot;',
	        "'": '&#39;'
	      })[char]);
	    }

	    function renderStats(stats = {}) {
	      statGrid.innerHTML = statOrder.map(([key, label]) => (
	        '<article class="card tone-' + key + '">' +
	          '<div class="label">' + label + '</div>' +
	          '<div class="value">' + (stats[key.charAt(0).toUpperCase() + key.slice(1)] ?? 0) + '</div>' +
	        '</article>'
	      )).join('');
	    }

	    function renderMeta(data) {
	      const counters = data.counters ?? {};
	      const buffers = data.buffers ?? {};
	      metaRail.innerHTML = [
	        ['Uptime', escapeHtml(data.uptime ?? '0s')],
	        ['Finished', counters.finished ?? 0],
	        ['Retries', counters.retries ?? 0],
	        ['Webhooks', (counters.hookOK ?? 0) + ' ok / ' + (counters.hookFail ?? 0) + ' failed'],
	        ['Cmdhooks', (counters.cmdOK ?? 0) + ' ok / ' + (counters.cmdFail ?? 0) + ' failed'],
	        ['Buffers', 'fs ' + (buffers.folderEvents ?? 0) + ', updates ' + ((buffers.xtractUpdates ?? 0) + (buffers.folderUpdates ?? 0)) + ', hooks ' + (buffers.hooks ?? 0) + ', deletes ' + (buffers.deletes ?? 0)]
	      ].map(([label, value]) => '<span class="pill"><strong>' + escapeHtml(label) + '</strong>' + escapeHtml(value) + '</span>').join('');
	    }

	    function renderDetailField(label, value, monospace) {
	      return '<div class="detail-field">' +
	        '<div class="label">' + escapeHtml(label) + '</div>' +
	        '<div class="value' + (monospace ? ' path' : '') + '">' + escapeHtml(value) + '</div>' +
	      '</div>';
	    }

	    function renderDetailList(title, values, monospace) {
	      return '<div class="detail-list">' +
	        '<strong>' + escapeHtml(title) + '</strong>' +
	        '<ul>' +
	          values.map((value) => '<li>' + (monospace ? '<code>' + escapeHtml(value) + '</code>' : escapeHtml(value)) + '</li>').join('') +
	        '</ul>' +
	      '</div>';
	    }

	    function renderTaskDetails(item) {
	      if (!item) {
	        detailCard.hidden = true;
	        return;
	      }

	      const details = item.details ?? {};
	      const progress = item.progress ?? null;
	      const fields = [
	        renderDetailField('Item', item.name, false),
	        renderDetailField('App', item.app, false),
	        renderDetailField('Status', item.statusText, false),
	        renderDetailField('Updated', new Date(item.updatedAt).toLocaleString(), false),
	        renderDetailField('Age', item.elapsed, false),
	        renderDetailField('Path', item.path, true)
	      ];

	      if (details.title) fields.push(renderDetailField('Title', details.title, false));
	      if (item.currentFile) fields.push(renderDetailField('Current archive', item.currentFile, true));
	      if (progress && progress.summary) fields.push(renderDetailField('Progress', progress.summary, false));
	      if (details.bytes) fields.push(renderDetailField('Bytes written', details.bytes, false));
	      if (details.elapsed) fields.push(renderDetailField('Extract elapsed', details.elapsed, false));
	      if (details.startedAt) fields.push(renderDetailField('Extract started', new Date(details.startedAt).toLocaleString(), false));
	      if (details.output) fields.push(renderDetailField('Temp folder', details.output, true));
	      if (details.queue) fields.push(renderDetailField('Queue at start', details.queue, false));
	      if (item.reason) fields.push(renderDetailField('Reason', item.reason, false));
	      if (item.error) fields.push(renderDetailField('Error', item.error, false));

	      const lists = [];
	      if ((details.archives ?? []).length) {
	        lists.push(renderDetailList('Archives', details.archives, true));
	      }
	      if ((details.files ?? []).length) {
	        lists.push(renderDetailList('Extracted files', details.files, true));
	      }
	      if (details.ids && Object.keys(details.ids).length) {
	        const pairs = Object.entries(details.ids).map(([key, value]) => key + ': ' + value);
	        lists.push(renderDetailList('Metadata', pairs, false));
	      }

	      detailGrid.innerHTML = fields.join('');
	      detailLists.innerHTML = lists.join('');
	      detailSubtitle.textContent = item.completed
	        ? 'Completed task retained in the UI until you clear it.'
	        : 'Live task details and extraction context.';
	      detailCard.hidden = false;
	    }

	    function renderItems(data) {
	      const items = data.items ?? [];
	      const activeCount = data.activeCount ?? 0;
	      const completedCount = data.completedCount ?? 0;
	      itemCount.textContent = activeCount + ' active / ' + completedCount + ' completed';
	      clearCompletedButton.disabled = completedCount === 0;

	      if (!items.length) {
	        itemsBody.innerHTML = '<tr><td colspan="5" class="empty">Nothing is queued or being tracked right now.</td></tr>';
	        detailCard.hidden = true;
	        return;
	      }

	      itemsBody.innerHTML = items.map((item) => {
	        const progress = item.progress;
	        const progressHtml = progress ? (
	          '<div class="progress">' +
	            '<div><strong>' + escapeHtml((progress.percent || 0).toFixed(0)) + '%</strong> / ' + escapeHtml(progress.archiveIndex || 1) + ' of ' + escapeHtml(progress.archiveCount || 1) + ' archive' + ((progress.archiveCount || 1) === 1 ? '' : 's') + '</div>' +
	            '<div class="progress-bar"><span style="width:' + Math.max(0, Math.min(100, progress.percent || 0)) + '%"></span></div>' +
	            '<div class="muted">' + escapeHtml((progress.percent || 0).toFixed(0)) + '% - ' + escapeHtml(progress.writtenBytes || '0B') + ' / ' + escapeHtml(progress.totalBytes || '0B') + '</div>' +
	            (progress.archive ? '<div class="path">' + escapeHtml(progress.archive) + '</div>' : '') +
	          '</div>'
	        ) : '<span class="muted">' + (item.completed ? 'Click for extracted-file details' : 'No byte-level progress yet') + '</span>';

	        const notes = [
	          item.reason ? '<div class="muted"><strong>Reason</strong> ' + escapeHtml(item.reason) + '</div>' : '',
	          item.error ? '<div class="muted"><strong>Error</strong> ' + escapeHtml(item.error) + '</div>' : '',
	          item.completed ? '<div class="muted">Retained until cleared</div>' : ''
	        ].join('');

	        return (
	          '<tr class="items-row' + (selectedItemId === item.id ? ' is-selected' : '') + '" data-item-id="' + escapeHtml(item.id) + '">' +
	            '<td>' +
	              '<div><strong>' + escapeHtml(item.name) + '</strong></div>' +
	              '<div class="muted">' + escapeHtml(item.app) + '</div>' +
	              notes +
	            '</td>' +
	            '<td><span class="status ' + escapeHtml(item.status) + '">' + escapeHtml(item.statusText) + '</span></td>' +
	            '<td>' + progressHtml + '</td>' +
	            '<td>' +
	              '<div>' + escapeHtml(item.elapsed) + '</div>' +
	              '<div class="muted">' + escapeHtml(new Date(item.updatedAt).toLocaleString()) + '</div>' +
	            '</td>' +
	            '<td><div class="path">' + escapeHtml(item.path) + '</div></td>' +
	          '</tr>'
	        );
	      }).join('');
	    }

	    function renderSnapshot(data) {
	      lastSnapshot = data ?? { items: [] };
	      renderStats(lastSnapshot.stats);
	      renderMeta(lastSnapshot);
	      renderItems(lastSnapshot);

	      if (selectedItemId) {
	        const selectedItem = (lastSnapshot.items ?? []).find((item) => item.id === selectedItemId);
	        if (selectedItem) {
	          renderTaskDetails(selectedItem);
	        } else {
	          selectedItemId = '';
	          detailCard.hidden = true;
	        }
	      }
	    }

	    async function refresh() {
	      try {
	        const response = await fetch(statusUrl, { cache: 'no-store' });
	        if (!response.ok) throw new Error('HTTP ' + response.status);
	        const data = await response.json();
	        renderSnapshot(data);
	        const generated = data.generatedAt ? new Date(data.generatedAt).toLocaleString() : 'just now';
	        stamp.textContent = 'Updated ' + generated;
	      } catch (error) {
	        stamp.textContent = 'Status unavailable: ' + error.message;
	      }
	    }

	    detailClose.addEventListener('click', () => {
	      selectedItemId = '';
	      detailCard.hidden = true;
	      renderItems(lastSnapshot);
	    });

	    clearCompletedButton.addEventListener('click', async () => {
	      try {
	        const response = await fetch(clearCompletedUrl, { method: 'POST', cache: 'no-store' });
	        if (!response.ok) throw new Error('HTTP ' + response.status);
	        const data = await response.json();
	        if (selectedItemId && !(data.items ?? []).some((item) => item.id === selectedItemId)) {
	          selectedItemId = '';
	          detailCard.hidden = true;
	        }
	        renderSnapshot(data);
	        stamp.textContent = 'Cleared completed items';
	      } catch (error) {
	        stamp.textContent = 'Unable to clear completed items: ' + error.message;
	      }
	    });

	    itemsBody.addEventListener('click', (event) => {
	      const row = event.target.closest('tr[data-item-id]');
	      if (!row) {
	        return;
	      }

	      const item = (lastSnapshot.items ?? []).find((candidate) => candidate.id === row.dataset.itemId);
	      if (!item) {
	        return;
	      }

	      selectedItemId = item.id;
	      renderItems(lastSnapshot);
	      renderTaskDetails(item);
	    });

	    refresh();
	    setInterval(refresh, 2000);
	  </script>
</body>
</html>`))
