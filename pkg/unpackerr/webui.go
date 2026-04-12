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
	GeneratedAt string            `json:"generatedAt"`
	Uptime      string            `json:"uptime"`
	Stats       *Stats            `json:"stats"`
	Buffers     webStatusBuffers  `json:"buffers"`
	Counters    webStatusCounters `json:"counters"`
	Items       []webStatusItem   `json:"items"`
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
	App         string             `json:"app"`
	CurrentFile string             `json:"currentFile,omitempty"`
	Elapsed     string             `json:"elapsed"`
	Error       string             `json:"error,omitempty"`
	Name        string             `json:"name"`
	Path        string             `json:"path"`
	Progress    *webStatusProgress `json:"progress,omitempty"`
	Reason      string             `json:"reason,omitempty"`
	Status      string             `json:"status"`
	StatusText  string             `json:"statusText"`
	UpdatedAt   string             `json:"updatedAt"`
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
	stats := u.currentStats()
	items := make([]webStatusItem, 0, len(u.Map))

	for name, item := range u.Map {
		items = append(items, buildWebStatusItem(name, item, now))
	}

	if u.folders != nil {
		for name, folder := range u.folders.Folders {
			if _, ok := u.Map[name]; ok || folder == nil {
				continue
			}

			items = append(items, buildWaitingFolderStatusItem(name, folder, now))
		}
	}

	sort.Slice(items, func(i, j int) bool {
		left, right := items[i], items[j]
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

	return &webStatusSnapshot{
		GeneratedAt: now.Format(time.RFC3339),
		Uptime:      durafmt.Parse(now.Sub(version.Started)).LimitFirstN(3).Format(durafmtUnits),
		Stats:       stats,
		Buffers:     buffers,
		Counters: webStatusCounters{
			CmdFail:  stats.CmdFail,
			CmdOK:    stats.CmdOK,
			Finished: u.Finished,
			HookFail: stats.HookFail,
			HookOK:   stats.HookOK,
			Retries:  u.Retries,
		},
		Items: items,
	}
}

func buildWebStatusItem(name string, item *Extract, now time.Time) webStatusItem {
	output := webStatusItem{
		App:        string(item.App),
		Elapsed:    now.Sub(item.Updated).Round(time.Second).String(),
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

var statusTemplate = template.Must(template.New("status").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Unpackerr Status</title>
  <style>
    :root {
      color-scheme: dark;
      --bg-top: #0d1419;
      --bg-bottom: #15222d;
      --panel: rgba(20, 31, 41, 0.78);
      --panel-strong: rgba(27, 41, 53, 0.92);
      --panel-soft: rgba(255, 255, 255, 0.07);
      --panel-softer: rgba(255, 255, 255, 0.04);
      --text: #edf4f8;
      --heading: #ffffff;
      --muted: #9eb2c0;
      --accent: #ffb84d;
      --accent-strong: #ff9453;
      --good: #54d293;
      --warn: #ffb84d;
      --bad: #ff6b6b;
      --border: rgba(255, 255, 255, 0.1);
      --border-strong: rgba(255, 255, 255, 0.18);
      --shadow: 0 20px 52px rgba(0, 0, 0, 0.28);
      --font: "Segoe UI Variable Display", "Aptos", "Segoe UI", sans-serif;
      --mono: Consolas, "SFMono-Regular", monospace;
    }
    html[data-theme="light"] {
      color-scheme: light;
      --bg-top: #f7efe4;
      --bg-bottom: #eef4f6;
      --panel: rgba(255, 255, 255, 0.82);
      --panel-strong: rgba(255, 255, 255, 0.96);
      --panel-soft: rgba(21, 34, 45, 0.05);
      --panel-softer: rgba(21, 34, 45, 0.03);
      --text: #173041;
      --heading: #122330;
      --muted: #63798a;
      --accent: #cf6f16;
      --accent-strong: #ad5311;
      --good: #1e8b60;
      --warn: #c98210;
      --bad: #c44949;
      --border: rgba(21, 34, 45, 0.11);
      --border-strong: rgba(21, 34, 45, 0.18);
      --shadow: 0 18px 38px rgba(29, 55, 75, 0.12);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      font-family: var(--font);
      background:
        radial-gradient(circle at top left, rgba(255, 184, 77, 0.2), transparent 28%),
        radial-gradient(circle at top right, rgba(84, 210, 147, 0.13), transparent 24%),
        linear-gradient(180deg, var(--bg-top), var(--bg-bottom));
      color: var(--text);
      transition: background 160ms ease, color 160ms ease;
    }
    .shell {
      max-width: 1200px;
      margin: 0 auto;
      padding: 28px 18px 52px;
    }
    .hero {
      display: grid;
      gap: 16px;
      margin-bottom: 22px;
      padding: 22px;
      border: 1px solid var(--border);
      border-radius: 24px;
      background: linear-gradient(180deg, var(--panel), var(--panel-strong));
      box-shadow: var(--shadow);
      backdrop-filter: blur(12px);
    }
    .headline {
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
      align-items: flex-start;
      justify-content: space-between;
    }
    .headline-actions {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      align-items: center;
      justify-content: flex-end;
    }
    h1 {
      margin: 0;
      font-size: clamp(2rem, 4.8vw, 3.2rem);
      line-height: 0.96;
      letter-spacing: -0.03em;
      color: var(--heading);
    }
    .subtle {
      color: var(--muted);
      font-size: 0.96rem;
      line-height: 1.5;
    }
    .theme-toggle {
      appearance: none;
      border: 1px solid var(--border-strong);
      background: var(--panel-soft);
      color: var(--text);
      border-radius: 999px;
      padding: 10px 14px;
      font: inherit;
      font-size: 0.9rem;
      cursor: pointer;
      transition: transform 140ms ease, border-color 140ms ease, background 140ms ease;
    }
    .theme-toggle:hover {
      transform: translateY(-1px);
      border-color: var(--accent);
      background: rgba(255, 184, 77, 0.12);
    }
    .grid {
      display: grid;
      gap: 14px;
      grid-template-columns: repeat(auto-fit, minmax(135px, 1fr));
    }
    .card, .table-card {
      background: linear-gradient(180deg, var(--panel), var(--panel-strong));
      border: 1px solid var(--border);
      border-radius: 20px;
      box-shadow: var(--shadow);
      backdrop-filter: blur(8px);
    }
    .card {
      padding: 18px;
      position: relative;
      overflow: hidden;
    }
    .card::before {
      content: "";
      position: absolute;
      inset: 0 auto 0 0;
      width: 4px;
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
      margin-top: 12px;
      color: var(--heading);
      font-size: 2.2rem;
      font-weight: 800;
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
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      margin-top: 8px;
    }
    .pill {
      background: var(--panel-soft);
      border: 1px solid var(--border);
      color: var(--muted);
      border-radius: 999px;
      padding: 8px 12px;
      font-size: 0.88rem;
    }
    .pill strong {
      color: var(--text);
      font-weight: 700;
      margin-right: 6px;
    }
    .table-card {
      overflow: hidden;
      margin-top: 18px;
    }
    .table-head {
      display: flex;
      justify-content: space-between;
      gap: 12px;
      align-items: center;
      padding: 18px 18px 12px;
    }
    .table-wrap {
      overflow-x: auto;
      padding: 0 10px 10px;
    }
    table {
      width: 100%;
      border-collapse: collapse;
      min-width: 760px;
    }
    th, td {
      text-align: left;
      padding: 16px 14px;
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
      background: var(--panel-softer);
    }
    .status {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      background: var(--panel-soft);
      border: 1px solid transparent;
      border-radius: 999px;
      padding: 7px 12px;
      white-space: nowrap;
      font-size: 0.86rem;
    }
    .status::before {
      content: "";
      width: 9px;
      height: 9px;
      border-radius: 50%;
      background: currentColor;
      box-shadow: 0 0 0 4px rgba(255,255,255,0.06);
    }
    .status.extracting {
      color: var(--accent);
      background: rgba(255, 184, 77, 0.14);
      border-color: rgba(255, 184, 77, 0.2);
    }
    .status.queued,
    .status.waiting {
      color: var(--warn);
      background: rgba(255, 184, 77, 0.14);
      border-color: rgba(255, 184, 77, 0.2);
    }
    .status.extractfailed,
    .status.deletefailed {
      color: var(--bad);
      background: rgba(255, 107, 107, 0.14);
      border-color: rgba(255, 107, 107, 0.2);
    }
    .status.extracted,
    .status.imported,
    .status.deleted,
    .status.deleting {
      color: var(--good);
      background: rgba(84, 210, 147, 0.14);
      border-color: rgba(84, 210, 147, 0.2);
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
      background: var(--panel-soft);
      border: 1px solid var(--border);
      overflow: hidden;
    }
    .progress-bar > span {
      position: relative;
      display: block;
      height: 100%;
      background: linear-gradient(90deg, var(--accent), var(--accent-strong));
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
      padding: 36px 18px 40px;
      text-align: center;
      color: var(--muted);
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
        padding-top: 22px;
      }
      .headline-actions {
        justify-content: flex-start;
      }
      .table-head {
        display: grid;
      }
    }
  </style>
</head>
<body>
  <div class="shell">
    <section class="hero">
      <div class="headline">
        <div>
          <h1>Unpackerr Status</h1>
          <div class="subtle">Live extraction progress without opening Dozzle.</div>
        </div>
        <div class="headline-actions">
          <button class="theme-toggle" id="theme-toggle" type="button">Use light theme</button>
          <div class="subtle" id="stamp">Loading...</div>
        </div>
      </div>
      <div class="grid" id="stat-grid"></div>
      <div class="rail" id="meta-rail"></div>
    </section>
    <section class="table-card">
      <div class="table-head">
        <div>
          <strong>Tracked Items</strong>
          <div class="subtle">Current queue, active extractions, and recent post-extract states.</div>
        </div>
        <div class="subtle" id="item-count"></div>
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
  </div>
  <script>
    const statOrder = [
      ["extracting", "Extracting"],
      ["queued", "Queued"],
      ["waiting", "Waiting"],
      ["failed", "Failed"],
      ["extracted", "Extracted"],
      ["imported", "Imported"],
      ["deleted", "Deleted"]
    ];

    const statGrid = document.getElementById("stat-grid");
    const metaRail = document.getElementById("meta-rail");
    const stamp = document.getElementById("stamp");
    const themeToggle = document.getElementById("theme-toggle");
    const itemCount = document.getElementById("item-count");
    const itemsBody = document.getElementById("items");
    const statusUrl = new URL("api/status", window.location.href);
    const themeKey = "unpackerr-theme";

    function escapeHtml(value) {
      return String(value ?? "").replace(/[&<>"']/g, (char) => ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        "\"": "&quot;",
        "'": "&#39;"
      })[char]);
    }

    function applyTheme(theme) {
      document.documentElement.setAttribute("data-theme", theme);
      themeToggle.textContent = theme === "light" ? "Use dark theme" : "Use light theme";
    }

    function initTheme() {
      const savedTheme = localStorage.getItem(themeKey);
      if (savedTheme === "light" || savedTheme === "dark") {
        applyTheme(savedTheme);
        return;
      }

      applyTheme(window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark");
    }

    themeToggle.addEventListener("click", () => {
      const nextTheme = document.documentElement.getAttribute("data-theme") === "light" ? "dark" : "light";
      localStorage.setItem(themeKey, nextTheme);
      applyTheme(nextTheme);
    });

    function renderStats(stats = {}) {
      statGrid.innerHTML = statOrder.map(([key, label]) => (
        '<article class="card tone-' + key + '">' +
          '<div class="label">' + label + '</div>' +
          '<div class="value">' + (stats[key.charAt(0).toUpperCase() + key.slice(1)] ?? 0) + '</div>' +
        '</article>'
      )).join("");
    }

    function renderMeta(data) {
      const counters = data.counters ?? {};
      const buffers = data.buffers ?? {};
      metaRail.innerHTML = [
        ['Uptime', escapeHtml(data.uptime ?? "0s")],
        ['Finished', counters.finished ?? 0],
        ['Retries', counters.retries ?? 0],
        ['Webhooks', (counters.hookOK ?? 0) + ' ok / ' + (counters.hookFail ?? 0) + ' failed'],
        ['Cmdhooks', (counters.cmdOK ?? 0) + ' ok / ' + (counters.cmdFail ?? 0) + ' failed'],
        ['Buffers', 'fs ' + (buffers.folderEvents ?? 0) + ', updates ' + ((buffers.xtractUpdates ?? 0) + (buffers.folderUpdates ?? 0)) + ', hooks ' + (buffers.hooks ?? 0) + ', deletes ' + (buffers.deletes ?? 0)]
      ].map(([label, value]) => '<span class="pill"><strong>' + escapeHtml(label) + '</strong>' + escapeHtml(value) + '</span>').join("");
    }

    function renderItems(items = []) {
      itemCount.textContent = items.length + ' tracked item' + (items.length === 1 ? "" : "s");
      if (!items.length) {
        itemsBody.innerHTML = '<tr><td colspan="5" class="empty">Nothing is queued or being tracked right now.</td></tr>';
        return;
      }

      itemsBody.innerHTML = items.map((item) => {
        const progress = item.progress;
        const progressHtml = progress ? (
          '<div class="progress">' +
            '<div><strong>' + escapeHtml((progress.percent || 0).toFixed(0)) + '%</strong> / ' + escapeHtml(progress.archiveIndex || 1) + ' of ' + escapeHtml(progress.archiveCount || 1) + ' archive' + ((progress.archiveCount || 1) === 1 ? "" : "s") + '</div>' +
            '<div class="progress-bar"><span style="width:' + Math.max(0, Math.min(100, progress.percent || 0)) + '%"></span></div>' +
            '<div class="muted">' + escapeHtml((progress.percent || 0).toFixed(0)) + '% - ' + escapeHtml(progress.writtenBytes || "0B") + ' / ' + escapeHtml(progress.totalBytes || "0B") + '</div>' +
            (progress.archive ? '<div class="path">' + escapeHtml(progress.archive) + '</div>' : '') +
          '</div>'
        ) : '<span class="muted">No byte-level progress yet</span>';

        const reasons = [
          item.reason ? '<div class="muted"><strong>Reason</strong> ' + escapeHtml(item.reason) + '</div>' : '',
          item.error ? '<div class="muted"><strong>Error</strong> ' + escapeHtml(item.error) + '</div>' : ''
        ].join("");

        return (
          '<tr>' +
            '<td>' +
              '<div><strong>' + escapeHtml(item.name) + '</strong></div>' +
              '<div class="muted">' + escapeHtml(item.app) + '</div>' +
              reasons +
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
      }).join("");
    }

    async function refresh() {
      try {
        const response = await fetch(statusUrl, { cache: "no-store" });
        if (!response.ok) throw new Error('HTTP ' + response.status);
        const data = await response.json();
        renderStats(data.stats);
        renderMeta(data);
        renderItems(data.items);
        const generated = data.generatedAt ? new Date(data.generatedAt).toLocaleString() : "just now";
        stamp.textContent = 'Updated ' + generated;
      } catch (error) {
        stamp.textContent = 'Status unavailable: ' + error.message;
      }
    }

    initTheme();
    refresh();
    setInterval(refresh, 2000);
  </script>
</body>
</html>`))
