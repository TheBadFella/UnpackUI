<script lang="ts">
  import { _ } from '../lib/i18n/Translate.svelte'
  import { statusPhrase, bytes, relTime, isZeroTime, remainCompact } from '../lib/format'
  import {
    itemTitle,
    deleteRemaining,
    formatRate,
    formatEta,
    visibleFiles,
  } from '../lib/dashboard'
  import type { DashboardRecord } from '../lib/dashboard'

  let {
    item,
    now = Date.now(),
    onclose,
  }: {
    item: DashboardRecord
    now?: number
    onclose: () => void
  } = $props()

  function normalizePath(value?: string): string {
    return String(value ?? '').replace(/\\/g, '/').replace(/\/+$/, '')
  }

  function pathBase(value?: string): string {
    const normalized = normalizePath(value)
    if (!normalized) return ''
    const parts = normalized.split('/')
    return parts[parts.length - 1] || normalized
  }

  function pathDir(value?: string): string {
    const text = String(value ?? '')
    const index = Math.max(text.lastIndexOf('\\'), text.lastIndexOf('/'))
    return index > -1 ? text.slice(0, index) : ''
  }

  function relativePath(value?: string, root?: string): string {
    const normalizedValue = normalizePath(value)
    const normalizedRoot = normalizePath(root)
    if (
      !normalizedValue ||
      !normalizedRoot ||
      !normalizedValue.startsWith(normalizedRoot + '/')
    ) {
      return pathBase(value) || (value ?? '')
    }
    return normalizedValue.slice(normalizedRoot.length + 1)
  }

  const title = $derived(itemTitle(item))
  const location = $derived(normalizePath(item.path))
  const sourceArchive = $derived.by(() => {
    if ('sourceArchive' in item && item.sourceArchive) {
      return normalizePath(item.sourceArchive)
    }
    if ('archive' in item && item.archive) {
      return normalizePath(item.archive)
    }
    return ''
  })
  const outputPath = $derived(
    'outputPath' in item && item.outputPath ? normalizePath(item.outputPath) : ''
  )
  const currentArchive = $derived.by(() => {
    if ('currentArchive' in item && item.currentArchive) {
      return normalizePath(item.currentArchive)
    }
    if ('archive' in item && item.archive) {
      return normalizePath(item.archive)
    }
    return ''
  })
  const isCompleted = $derived(
    item.status === 'finished' ||
      item.status === 'extracted' ||
      item.status === 'imported' ||
      item.status === 'deleted' ||
      item.status === 'extractfailed' ||
      ('finished' in item && !!item.finished && !isZeroTime(item.finished))
  )

  const updatedAtFormatted = $derived(
    item.updated && !isZeroTime(item.updated)
      ? new Date(item.updated).toLocaleString()
      : ''
  )
  const startedAtFormatted = $derived(
    item.started && !isZeroTime(item.started)
      ? new Date(item.started).toLocaleString()
      : ''
  )
  const finishedAtFormatted = $derived(
    'finished' in item && item.finished && !isZeroTime(item.finished)
      ? new Date(item.finished).toLocaleString()
      : ''
  )
  const deleteAtFormatted = $derived(
    item.deleteAt ? new Date(item.deleteAt).toLocaleString() : ''
  )
  const deletesIn = $derived.by(() => {
    if ('dueKind' in item && item.dueKind === 'cleanup' && item.due && !isZeroTime(item.due)) {
      return remainCompact(item.due, now) || deleteRemaining(item.deleteAt, now)
    }
    return deleteRemaining(item.deleteAt, now)
  })

  const progressSummary = $derived.by(() => {
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
    if ('eta' in item && item.eta && !isZeroTime(item.eta)) {
      return remainCompact(item.eta, now)
    }
    if ('etaSeconds' in item && item.etaSeconds) {
      return formatEta(item.etaSeconds)
    }
    return ''
  })
  const elapsed = $derived(
    'elapsed' in item && item.elapsed
      ? item.elapsed
      : item.updated
        ? relTime(item.updated, now)
        : ''
  )

  const archives = $derived.by(() => {
    if ('archiveFiles' in item && Array.isArray(item.archiveFiles) && item.archiveFiles.length) {
      return visibleFiles(item.archiveFiles).map((archive) => relativePath(archive, location))
    }
    if ('archive' in item && item.archive) {
      return [relativePath(item.archive, location)]
    }
    return []
  })

  const extractedFiles = $derived.by(() => {
    if ('newFiles' in item && Array.isArray(item.newFiles)) {
      const targetRoot = outputPath || location
      return visibleFiles(item.newFiles).map((f) => relativePath(f, targetRoot))
    }
    return []
  })

  const hasExtractionDetails = $derived(
    !!(progressSummary || speed || eta || elapsed || startedAtFormatted || finishedAtFormatted)
  )
</script>

<section class="table-card detail-card" id="detail-card">
  <div class="table-head">
    <div>
      <strong>Task Details</strong>
      <div class="subtle">
        {isCompleted
          ? 'Completed task summary.'
          : 'Live task details and extraction context.'}
      </div>
    </div>
    <div class="headline-actions">
      <button class="action-button" type="button" onclick={onclose}>Close</button>
    </div>
  </div>
  <div class="table-wrap">
    <div class="detail-grid">
      <!-- Section 1: Overview (wide) -->
      <div class="detail-section detail-section-wide">
        <div class="detail-section-title">{title}</div>
        <div class="detail-meta">
          <div class="detail-meta-item">
            <div class="detail-label">Status</div>
            <div class="detail-value">{$_(statusPhrase(item.status))}</div>
          </div>
          <div class="detail-meta-item">
            <div class="detail-label">Application</div>
            <div class="detail-value">{item.app}</div>
          </div>
          {#if updatedAtFormatted}
            <div class="detail-meta-item">
              <div class="detail-label">Updated</div>
              <div class="detail-value">{updatedAtFormatted}</div>
            </div>
          {/if}
          {#if !isCompleted && elapsed}
            <div class="detail-meta-item">
              <div class="detail-label">Age</div>
              <div class="detail-value">{elapsed}</div>
            </div>
          {/if}
          {#if item.error}
            <div class="detail-meta-item">
              <div class="detail-label">Error</div>
              <div class="detail-value text-danger">{item.error}</div>
            </div>
          {/if}
        </div>
        <div class="detail-paths">
          {#if location}
            <div class="detail-path-row">
              <div class="detail-label">Location</div>
              <div class="detail-path-value">{location}</div>
            </div>
          {/if}
          {#if item.reason}
            <div class="detail-path-row">
              <div class="detail-label">Reason</div>
              <div class="detail-path-value">{item.reason}</div>
            </div>
          {/if}
          {#if sourceArchive}
            <div class="detail-path-row detail-path-row-wide">
              <div class="detail-label">Source archive</div>
              <div class="detail-path-value">{sourceArchive}</div>
            </div>
          {/if}
          {#if outputPath}
            <div class="detail-path-row detail-path-row-wide">
              <div class="detail-label">Output folder</div>
              <div class="detail-path-value">{outputPath}</div>
            </div>
          {/if}
          {#if item.url}
            <div class="detail-path-row detail-path-row-wide">
              <div class="detail-label">Source URL</div>
              <div class="detail-path-value">{item.url}</div>
            </div>
          {/if}
          {#if currentArchive}
            <div class="detail-path-row detail-path-row-wide">
              <div class="detail-label">Current archive</div>
              <div class="detail-path-value">{currentArchive}</div>
            </div>
          {/if}
        </div>
      </div>

      <!-- Section 2: Extraction -->
      <div class="detail-section">
        <div class="detail-label">Extraction</div>
        {#if hasExtractionDetails}
          <div class="detail-lines">
            {#if progressSummary}
              <div class="detail-value">{progressSummary}</div>
            {/if}
            {#if speed}
              <div class="detail-meta-item">
                <div class="detail-label">Speed</div>
                <div class="detail-value">{speed}</div>
              </div>
            {/if}
            {#if eta}
              <div class="detail-meta-item">
                <div class="detail-label">ETA</div>
                <div class="detail-value">{eta}</div>
              </div>
            {/if}
            {#if elapsed}
              <div class="detail-meta-item">
                <div class="detail-label">Elapsed</div>
                <div class="detail-value">{elapsed}</div>
              </div>
            {/if}
            {#if startedAtFormatted}
              <div class="detail-meta-item">
                <div class="detail-label">Started</div>
                <div class="detail-value">{startedAtFormatted}</div>
              </div>
            {/if}
            {#if finishedAtFormatted}
              <div class="detail-meta-item">
                <div class="detail-label">Finished</div>
                <div class="detail-value">{finishedAtFormatted}</div>
              </div>
            {/if}
          </div>
        {:else}
          <div class="detail-value text-muted">No extraction details yet.</div>
        {/if}
      </div>

      <!-- Section 3: Cleanup -->
      <div class="detail-section">
        <div class="detail-label">Cleanup</div>
        {#if item.deleteAt}
          <div class="detail-lines">
            <div class="detail-meta-item">
              <div class="detail-label">Deletes in</div>
              <div class="detail-value">{deletesIn || 'due now'}</div>
            </div>
            <div class="detail-meta-item">
              <div class="detail-label">Scheduled for</div>
              <div class="detail-value">{deleteAtFormatted}</div>
            </div>
          </div>
        {:else if 'deleteDelay' in item && item.deleteDelay}
          <div class="detail-value">{item.deleteDelay}</div>
        {:else}
          <div class="detail-value text-muted">-</div>
        {/if}
      </div>
    </div>

    <!-- Section 4: File lists -->
    {#if archives.length || extractedFiles.length}
      <div class="detail-lists">
        {#if archives.length}
          <div class="detail-list">
            <strong>Archives ({archives.length})</strong>
            <ul>
              {#each archives as arch}
                <li><code>{arch}</code></li>
              {/each}
            </ul>
          </div>
        {/if}
        {#if extractedFiles.length}
          <div class="detail-list">
            <strong>Extracted files ({extractedFiles.length})</strong>
            <ul>
              {#each extractedFiles as file}
                <li><code>{file}</code></li>
              {/each}
            </ul>
          </div>
        {/if}
      </div>
    {/if}
  </div>
</section>
