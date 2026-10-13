<script lang="ts">
  import {
    Badge,
    Button,
    Modal,
    ModalBody,
    ModalFooter,
    ModalHeader,
    Tooltip,
  } from '@sveltestrap/sveltestrap'
  import { _ } from '../lib/i18n/Translate.svelte'
  import {
    bytes,
    dateTime,
    errorPhrase,
    isZeroTime,
    relTime,
    remainCompact,
    statusColor,
    statusPhrase,
  } from '../lib/format'
  import {
    itemTitle,
    itemReason,
    deleteRemaining,
    formatRate,
    formatEta,
    visibleFiles,
  } from '../lib/dashboard'
  import type { ItemMeta } from '../lib/types'
  import { slugifyPath } from '../lib/slug'
  import DownloadIcon from 'phosphor-svelte/lib/Download'
  import { theme } from '../lib/theme.svelte'

  let {
    item = null,
    now = Date.now(),
    loading = false,
    error = null,
    onclose,
  }: {
    item: ItemMeta | null
    now?: number
    loading?: boolean
    error?: string | null
    onclose: () => void
  } = $props()

  const uid = $props.id()

  const dueKeys: Record<string, string> = {
    start: 'pages.dashboard.DueStart',
    retry: 'pages.dashboard.DueRetry',
    cleanup: 'pages.dashboard.DueCleanup',
    history: 'pages.dashboard.DueHistory',
  }

  const ID_LABEL: Record<string, string> = {
    title: 'pages.detail.Title',
    downloadId: 'pages.detail.DownloadId',
    seriesId: 'pages.detail.SeriesId',
    episodeId: 'pages.detail.EpisodeId',
    movieId: 'pages.detail.MovieId',
    artistId: 'pages.detail.ArtistId',
    albumId: 'pages.detail.AlbumId',
    authorId: 'pages.detail.AuthorId',
    bookId: 'pages.detail.BookId',
    reason: 'pages.detail.Reason',
  }

  function idLabel(key: string): string {
    const loc = ID_LABEL[key]
    return loc ? $_(loc) : key
  }

  function normalizePath(value?: string): string {
    return String(value ?? '').replace(/\\/g, '/').replace(/\/+$/, '')
  }

  const title = $derived(item ? itemTitle(item) : '')
  const location = $derived(item ? normalizePath(item.path || item.id) : '')
  const sourceArchive = $derived.by(() => {
    if (!item) return ''
    if ('sourceArchive' in item && typeof item.sourceArchive === 'string' && item.sourceArchive) {
      return normalizePath(item.sourceArchive)
    }
    if ('archive' in item && item.archive) {
      return normalizePath(item.archive)
    }
    return ''
  })
  const outputPath = $derived.by(() => {
    if (!item) return ''
    if ('outputPath' in item && item.outputPath) return normalizePath(item.outputPath)
    if ('output' in item && item.output) return normalizePath(item.output)
    return ''
  })
  const currentArchive = $derived.by(() => {
    if (!item) return ''
    if ('currentArchive' in item && typeof item.currentArchive === 'string' && item.currentArchive) {
      return normalizePath(item.currentArchive)
    }
    if ('archive' in item && item.archive) {
      return normalizePath(item.archive)
    }
    return ''
  })

  const updatedAtFormatted = $derived(
    item?.updated && !isZeroTime(item.updated)
      ? dateTime(item.updated)
      : ''
  )
  const startedAtFormatted = $derived(
    item?.started && !isZeroTime(item.started)
      ? dateTime(item.started)
      : ''
  )
  const finishedAtFormatted = $derived(
    item && 'finished' in item && item.finished && !isZeroTime(item.finished)
      ? dateTime(item.finished)
      : ''
  )
  const deleteAtFormatted = $derived(
    item?.deleteAt && !isZeroTime(item.deleteAt) ? dateTime(item.deleteAt) : ''
  )

  const due = $derived(item ? dueText(item, now) : '')

  function dueText(row: ItemMeta, clock: number): string {
    if (!('dueKind' in row) || !row.dueKind || isZeroTime(row.due)) {
      if (row.deleteAt) return deleteRemaining(row.deleteAt, clock)
      return ''
    }
    const key = dueKeys[row.dueKind]
    if (!key) return ''
    const remain = remainCompact(row.due, clock)
    if (!remain) return $_('pages.dashboard.DueSoon')
    return $_(key, { values: { remain } })
  }

  function eventLabel(event: string): string {
    if (event === 'fsnotify') return $_('pages.dashboard.FSNotify')
    if (event === 'polling') return $_('pages.dashboard.Polling')
    return event
  }

  function eventHint(event: string): string {
    if (event === 'fsnotify') return $_('pages.dashboard.FSNotifyHint')
    if (event === 'polling') return $_('pages.dashboard.PollingHint')
    return event
  }

  const progressSummary = $derived.by(() => {
    if (!item) return ''
    if (
      'percent' in item &&
      (item.percent !== undefined || item.wrote !== undefined)
    ) {
      const pct = (item.percent ?? 0).toFixed(0)
      const curArch =
        'extracted' in item && item.extracted ? item.extracted : 1
      const totArch =
        'archives' in item && item.archives ? item.archives : 1
      const archLabel = totArch === 1 ? 'archive' : 'archives'
      const wrote = bytes(item.wrote ?? 0)
      const total = bytes(item.total ?? item.wrote ?? 0)
      return `${pct}% - ${curArch} of ${totArch} ${archLabel} - ${wrote} / ${total}`
    }
    if ('bytes' in item && item.bytes) {
      const archCount = ('archives' in item && item.archives) ? item.archives : 1
      const fileCount = ('files' in item && item.files) ? item.files : 0
      return `${bytes(item.bytes)} - ${archCount} ${archCount === 1 ? 'archive' : 'archives'}, ${fileCount} ${fileCount === 1 ? 'file' : 'files'}`
    }
    if (item.progress) return item.progress
    return ''
  })

  const speed = $derived.by(() => {
    if (!item) return ''
    if ('avgSpeedBps' in item && item.avgSpeedBps) {
      const avg = formatRate(item.avgSpeedBps)
      if ('speedBps' in item && item.speedBps && item.speedBps !== item.avgSpeedBps) {
        return `${avg} (current: ${formatRate(item.speedBps)})`
      }
      return avg
    }
    if ('speedBps' in item && item.speedBps) {
      return formatRate(item.speedBps)
    }
    if ('speedBytesPerSecond' in item && item.speedBytesPerSecond) {
      return formatRate(item.speedBytesPerSecond)
    }
    return ''
  })

  const eta = $derived.by(() => {
    if (!item) return ''
    if ('eta' in item && item.eta && !isZeroTime(item.eta)) {
      return remainCompact(item.eta, now)
    }
    if ('etaSeconds' in item && item.etaSeconds) {
      return formatEta(item.etaSeconds)
    }
    return ''
  })

  const elapsed = $derived.by(() => {
    if (!item) return ''
    if ('elapsed' in item && item.elapsed) return item.elapsed
    if (item.updated && !isZeroTime(item.updated)) return relTime(item.updated, now)
    return ''
  })

  const idRows = $derived.by(() => {
    if (!item?.ids) return []
    const path = item.path || item.id
    const folder = item.app === 'Folder'

    const rows = Object.entries(item.ids).filter(([key, value]) => {
      if (value === undefined || value === null || value === '') return false
      if (key === 'title' && (folder || String(value) === path || String(value) === title)) return false
      return true
    })

    const titleRow = rows.find(([key]) => key === 'title')
    if (!titleRow) return rows

    return [titleRow, ...rows.filter(([key]) => key !== 'title')]
  })

  const extraFiles = $derived(
    item && 'extraFiles' in item && Array.isArray(item.extraFiles) ? (item.extraFiles ?? []) : []
  )

  const origFiles = $derived.by(() => {
    if (!item) return []
    const raw: string[] = []
    if ('origFiles' in item && Array.isArray(item.origFiles)) raw.push(...item.origFiles)
    if ('archiveFiles' in item && Array.isArray(item.archiveFiles)) raw.push(...item.archiveFiles)
    if (extraFiles.length) raw.push(...extraFiles)
    if (!raw.length && 'archive' in item && item.archive) raw.push(item.archive)
    const seen = new Set<string>()
    const deduped: string[] = []
    for (const f of raw) {
      if (f && !seen.has(f)) {
        seen.add(f)
        deduped.push(f)
      }
    }
    return visibleFiles(deduped)
  })

  const newFiles = $derived.by(() => {
    if (!item) return []
    const raw = 'newFiles' in item && Array.isArray(item.newFiles) ? item.newFiles : []
    const seen = new Set<string>()
    const deduped: string[] = []
    for (const f of raw) {
      if (f && !seen.has(f)) {
        seen.add(f)
        deduped.push(f)
      }
    }
    return visibleFiles(deduped)
  })

  const hasOrig = $derived(origFiles.length > 0)
  const hasNew = $derived(newFiles.length > 0)

  function downloadFiles(files: string[] | undefined, kind: 'archives' | 'extracted') {
    if (!files || files.length === 0 || !item) return
    const baseSlug = slugifyPath(item.path || item.id || 'item') || 'item'
    const name = `${baseSlug}-${kind}.txt`
    const blob = new Blob([files.join('\n') + '\n'], {
      type: 'text/plain;charset=utf-8',
    })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = name
    document.body.append(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  }
</script>

<Modal
  isOpen={item !== null}
  toggle={onclose}
  size="xl"
  class="item-detail-modal"
  labelledBy="item-detail-title"
>
  {#if item}
    <ModalHeader toggle={onclose} id="item-detail-title">
      <div class="item-detail-title">
        <span class="d-inline-flex align-items-center gap-2 flex-wrap">
          <strong class="fs-5">{title || item.app}</strong>
          {#if title && title !== item.app}
            <span class="badge bg-secondary">{item.app}</span>
          {/if}
          <Badge color={statusColor(item.status)}>
            {$_(statusPhrase(item.status))}
          </Badge>
          {#if item.isUnavailable}
            <Badge color="secondary">
              {$_('pages.detail.Unavailable')}
            </Badge>
          {/if}
          <span class="visually-hidden">{item.path || item.id}</span>
        </span>
        {#if item.event}
          <Badge
            id="{uid}-event"
            class="item-detail-event"
            color={item.event === 'fsnotify' ? 'success' : 'warning'}
          >
            {eventLabel(item.event)}
          </Badge>
          <Tooltip target="{uid}-event" placement="left" theme={theme.tooltip}>
            {eventHint(item.event)}
          </Tooltip>
        {/if}
      </div>
    </ModalHeader>

    <ModalBody>
      {#if item.isUnavailable}
        <div class="alert alert-warning py-2 mb-3 d-flex align-items-center justify-content-between gap-2">
          <span>{$_('pages.detail.RecordUnavailable')}</span>
          <Button size="sm" color="secondary" outline type="button" onclick={onclose}>
            {$_('buttons.Close')}
          </Button>
        </div>
      {/if}

      {#if loading}
        <div class="alert alert-info py-2 mb-3 d-flex align-items-center gap-2">
          <span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span>
          <span>{$_('pages.detail.LoadingDetails')}</span>
        </div>
      {:else if error}
        <div class="alert alert-warning py-2 mb-3">
          {error}
        </div>
      {/if}

      <dl class="item-detail">
        {#if location}
          <dt>{$_('pages.detail.Path')}</dt>
          <dd><code class="wrap">{location}</code></dd>
        {/if}
        {#if outputPath && outputPath !== location}
          <dt>{$_('pages.detail.OutputPath')}</dt>
          <dd><code class="wrap">{outputPath}</code></dd>
        {/if}
        {#if sourceArchive && sourceArchive !== location}
          <dt>{$_('pages.detail.Archive')}</dt>
          <dd><code class="wrap">{sourceArchive}</code></dd>
        {/if}
        {#if currentArchive && currentArchive !== sourceArchive && currentArchive !== location}
          <dt>{$_('pages.detail.CurrentArchive')}</dt>
          <dd><code class="wrap">{currentArchive}</code></dd>
        {/if}
        {#if item.url}
          <dt>{$_('pages.detail.URL')}</dt>
          <dd><code class="wrap">{item.url}</code></dd>
        {/if}
        {#if item.kind && item.kind !== item.app}
          <dt>{$_('pages.detail.Kind')}</dt>
          <dd>{item.kind}</dd>
        {/if}
        {#if startedAtFormatted}
          <dt>{$_('pages.detail.Started')}</dt>
          <dd>{startedAtFormatted}</dd>
        {/if}
        {#if elapsed}
          <dt>{$_('pages.detail.Elapsed')}</dt>
          <dd>{elapsed}</dd>
        {/if}
        {#if finishedAtFormatted}
          <dt>{$_('pages.detail.Finished')}</dt>
          <dd>{finishedAtFormatted}</dd>
        {/if}
        {#if updatedAtFormatted}
          <dt>{$_('pages.detail.Updated')}</dt>
          <dd>{updatedAtFormatted}</dd>
        {/if}
        {#if progressSummary}
          <dt>{$_('pages.dashboard.Progress')}</dt>
          <dd>{progressSummary}</dd>
        {/if}
        {#if speed}
          <dt>{$_('pages.detail.Speed')}</dt>
          <dd>{speed}</dd>
        {/if}
        {#if eta}
          <dt>{$_('pages.dashboard.ETA')}</dt>
          <dd>{eta}</dd>
        {/if}
        {#if due}
          <dt>{$_('pages.detail.Due')}</dt>
          <dd>{due}</dd>
        {/if}
        {#if deleteAtFormatted}
          <dt>{$_('pages.detail.ScheduledCleanup')}</dt>
          <dd>{deleteAtFormatted}</dd>
        {/if}
        {#if 'deleteDelay' in item && item.deleteDelay}
          <dt>{$_('pages.detail.DeleteDelay')}</dt>
          <dd>{item.deleteDelay}</dd>
        {/if}
        {#if item.bytes}
          <dt>{$_('pages.detail.Size')}</dt>
          <dd>{bytes(item.bytes)}</dd>
        {/if}
        {#if item.ratio}
          <dt>{$_('pages.detail.Ratio')}</dt>
          <dd>{item.ratio.toFixed(2)}</dd>
        {/if}
        {#if item.archives || ('extracted' in item && item.extracted)}
          <dt>{$_('pages.detail.Archives')}</dt>
          <dd>
            {#if 'extracted' in item && item.extracted && item.archives}
              {$_('pages.dashboard.ArchiveOf', {
                values: { n: item.extracted, total: item.archives },
              })}
            {:else}
              {item.archives || 0}
            {/if}
          </dd>
        {/if}
        {#if item.files}
          <dt>{$_('pages.detail.Files')}</dt>
          <dd>{item.files}</dd>
        {/if}
        {#if item.queue}
          <dt>{$_('pages.detail.Queue')}</dt>
          <dd>{item.queue}</dd>
        {/if}
        {#if item.retries}
          <dt>{$_('pages.detail.Retries')}</dt>
          <dd>{item.retries}</dd>
        {/if}
        {#if item.hookFail}
          <dt>{$_('pages.detail.HookFail')}</dt>
          <dd>{item.hookFail}</dd>
        {/if}
        {#if itemReason(item)}
          <dt>{$_('pages.detail.Reason')}</dt>
          <dd>{itemReason(item)}</dd>
        {/if}
        {#if 'note' in item && item.note}
          <dt>{$_('pages.detail.Note')}</dt>
          <dd>{item.note}</dd>
        {/if}
        {#if item.error}
          {@const errKey = errorPhrase(item.error)}
          <dt>{$_('pages.detail.Error')}</dt>
          <dd class="text-danger">{errKey ? $_(errKey) : item.error}</dd>
        {/if}
        {#each idRows as [key, value] (key)}
          <dt>{idLabel(key)}</dt>
          <dd><code class="wrap">{String(value)}</code></dd>
        {/each}
      </dl>

      {#if hasOrig || hasNew}
        <div
          class="item-file-panes"
          class:item-file-panes-single={!hasOrig || !hasNew}
        >
          {#if hasOrig}
            <section class="item-file-pane">
              <div class="item-file-pane-head">
                <h6 id="item-files-orig">
                  {$_('pages.detail.ArchiveFiles')} ({origFiles.length})
                </h6>
                <Button
                  id="{uid}-dl-orig"
                  size="sm"
                  color="secondary"
                  outline
                  type="button"
                  class="item-file-download"
                  aria-label={$_('pages.detail.DownloadList', {
                    values: { name: $_('pages.detail.ArchiveFiles') },
                  })}
                  onclick={() => downloadFiles(origFiles, 'archives')}
                >
                  <DownloadIcon
                    size="1.25em"
                    weight="bold"
                    aria-hidden="true"
                    focusable="false"
                  />
                </Button>
                <Tooltip
                  target="{uid}-dl-orig"
                  placement="top"
                  theme={theme.tooltip}
                >
                  {$_('pages.detail.DownloadList', {
                    values: { name: $_('pages.detail.ArchiveFiles') },
                  })}
                </Tooltip>
              </div>
              <ul class="item-file-list" aria-labelledby="item-files-orig">
                {#each origFiles as file (file)}
                  <li><code>{file}</code></li>
                {/each}
              </ul>
            </section>
          {/if}
          {#if hasNew}
            <section class="item-file-pane">
              <div class="item-file-pane-head">
                <h6 id="item-files-new">
                  {$_('pages.detail.ExtractedFiles')} ({newFiles.length})
                </h6>
                <Button
                  id="{uid}-dl-new"
                  size="sm"
                  color="secondary"
                  outline
                  type="button"
                  class="item-file-download"
                  aria-label={$_('pages.detail.DownloadList', {
                    values: { name: $_('pages.detail.ExtractedFiles') },
                  })}
                  onclick={() => downloadFiles(newFiles, 'extracted')}
                >
                  <DownloadIcon
                    size="1.25em"
                    weight="bold"
                    aria-hidden="true"
                    focusable="false"
                  />
                </Button>
                <Tooltip
                  target="{uid}-dl-new"
                  placement="top"
                  theme={theme.tooltip}
                >
                  {$_('pages.detail.DownloadList', {
                    values: { name: $_('pages.detail.ExtractedFiles') },
                  })}
                </Tooltip>
              </div>
              <ul class="item-file-list" aria-labelledby="item-files-new">
                {#each newFiles as file (file)}
                  <li><code>{file}</code></li>
                {/each}
              </ul>
            </section>
          {/if}
        </div>
      {/if}
    </ModalBody>

    <ModalFooter>
      <Button color="secondary" type="button" onclick={onclose}>
        {$_('buttons.Close')}
      </Button>
    </ModalFooter>
  {/if}
</Modal>
