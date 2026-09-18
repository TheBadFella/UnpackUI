<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import {
    Badge,
    Button,
    ButtonGroup,
    Card,
    CardBody,
    CardTitle,
    Col,
    Row,
    Spinner,
    Table,
    Tooltip,
    Modal,
    ModalHeader,
    ModalBody,
    ModalFooter,
  } from '@sveltestrap/sveltestrap'
  import { _ } from '../lib/i18n/Translate.svelte'
  import { api } from '../lib/api'
  import { has } from '../lib/auth.svelte'
  import { systemPerm } from '../lib/perms'
  import {
    statusColor,
    statusPhrase,
    relTime,
    progressCaption,
    remainCompact,
    isZeroTime,
    bytes,
  } from '../lib/format'
  import {
    formatEta,
    formatRate,
    itemPath,
    itemReason,
    itemTitle,
    deleteRemaining,
  } from '../lib/dashboard'
  import {
    loadColumnWidths,
    resetColumnWidths,
    saveColumnWidths,
    colStyle,
    resizedColumnWidths,
    tableWidth,
  } from '../lib/columns'
  import { success, failure } from '../lib/toast'
  import { live, type LiveTopic } from '../lib/socket.svelte'
  import type { BufferStat, QueueItem } from '../lib/types'
  import History from './History.svelte'
  import TaskDetails from '../components/TaskDetails.svelte'

  let busy = $state<Record<string, boolean>>({})
  let now = $state(Date.now())
  let ageTimer: ReturnType<typeof setInterval> | undefined
  let pendingForget = $state<QueueItem | null>(null)
  let selectedQueueId = $state<string | null>(null)

  type QueueColumn =
    | 'item'
    | 'status'
    | 'progress'
    | 'deletes'
    | 'updated'
    | 'path'
    | 'actions'
  const queueColumnDefaults: Record<QueueColumn, string> = {
    item: '27%',
    status: '11%',
    progress: '17%',
    deletes: '9%',
    updated: '11%',
    path: '12%',
    actions: '13%',
  }
  const queueColumnMins: Record<QueueColumn, number> = {
    item: 180,
    status: 110,
    progress: 150,
    deletes: 110,
    updated: 130,
    path: 160,
    actions: 150,
  }
  const queueColumnKeys: QueueColumn[] = [
    'item',
    'status',
    'progress',
    'deletes',
    'updated',
    'path',
    'actions',
  ]
  const queueColumnStorageKey = 'unpackerr.dashboard.queue-columns.v1'
  let queueColumnWidths = $state<Record<QueueColumn, number | string>>({ ...queueColumnDefaults })
  let queueResize = $state<{
    key: QueueColumn
    startX: number
    startWidths: Record<QueueColumn, number>
    handle: HTMLElement
  } | null>(null)

  function resetQueueColumns() {
    queueColumnWidths = resetColumnWidths(
      queueColumnStorageKey,
      queueColumnDefaults,
    )
  }

  function startQueueResize(event: PointerEvent, key: QueueColumn) {
    event.preventDefault()
    const handle = event.currentTarget as HTMLElement
    try {
      handle.setPointerCapture(event.pointerId)
    } catch {}
    const heading = handle.parentElement
    if (!heading) return

    const thead = heading.parentElement
    const ths = thead ? (Array.from(thead.children) as HTMLElement[]) : []

    const startWidths: Record<QueueColumn, number> = {} as any
    ths.forEach((th) => {
      const colKey = th.dataset.columnKey as QueueColumn | undefined
      if (colKey && queueColumnKeys.includes(colKey)) {
        startWidths[colKey] = Math.round(th.getBoundingClientRect().width)
      }
    })

    queueResize = {
      key,
      startX: event.clientX,
      startWidths,
      handle,
    }
    document.body.classList.add('is-resizing-columns')
  }

  function moveQueueResize(event: PointerEvent | MouseEvent) {
    if (!queueResize) return
    const delta = event.clientX - queueResize.startX
    queueColumnWidths = resizedColumnWidths(
      queueResize.startWidths,
      queueResize.key,
      delta,
      queueColumnMins,
    )
  }

  function finishQueueResize(event: PointerEvent | MouseEvent) {
    if (!queueResize) return
    try {
      if ('pointerId' in event && queueResize.handle?.hasPointerCapture(event.pointerId)) {
        queueResize.handle.releasePointerCapture(event.pointerId)
      }
    } catch {}
    saveColumnWidths(queueColumnStorageKey, queueColumnWidths)
    queueResize = null
    document.body.classList.remove('is-resizing-columns')
  }

  function nudgeQueueResize(event: KeyboardEvent, key: QueueColumn) {
    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
    event.preventDefault()
    const delta = event.key === 'ArrowRight' ? 16 : -16
    const handle = event.currentTarget as HTMLElement
    const table = handle.closest('table')
    if (!table) return
    const measured: Record<QueueColumn, number> = {} as any
    table.querySelectorAll('thead th').forEach((th) => {
      const colKey = (th as HTMLElement).dataset.columnKey as QueueColumn | undefined
      if (colKey) measured[colKey] = Math.round((th as HTMLElement).getBoundingClientRect().width)
    })
    queueColumnWidths = resizedColumnWidths(measured, key, delta, queueColumnMins)
    saveColumnWidths(queueColumnStorageKey, queueColumnWidths)
  }

  const canQueue = has(systemPerm('queue', 'read'))
  const canWrite = has(systemPerm('queue', 'write'))
  const canStats = has(systemPerm('stats', 'read'))
  const canHistory = has(systemPerm('history', 'read'))

  const TERMINAL = [
    'extractfailed',
    'extractednothing',
    'imported',
    'deleted',
    'deletefailed',
  ]

  const uid = $props.id()
  const stats = $derived(live.stats)
  const queue = $derived(live.queue)
  const queueShowsDelete = $derived(queue.some((item) => Boolean(item.deleteAt)))
  const queueVisibleColumns = $derived(
    queueColumnKeys.filter((key) => key !== 'deletes' || queueShowsDelete),
  )
  const queueTableWidth = $derived(tableWidth(queueColumnWidths, queueVisibleColumns))
  const selectedQueueItem = $derived(
    queue.find((item) => item.id === selectedQueueId) ?? null
  )

  function toggleQueueSelect(id: string) {
    selectedQueueId = selectedQueueId === id ? null : id
  }
  // The badge belongs to the queue table below. History is rendered in its
  // own capped list and may contain the same durable terminal item, so adding
  // both collections would count some visible records twice.
  const trackedCount = $derived(queue.length)
  const loading = $derived(live.fetchedAt === undefined)

  const cards = $derived(
    stats
      ? [
          {
            id: 'waiting',
            label: $_('pages.dashboard.Waiting'),
            hint: $_('pages.dashboard.WaitingHint'),
            value: stats.waiting,
            color: 'secondary',
          },
          {
            id: 'queued',
            label: $_('pages.dashboard.Queued'),
            hint: $_('pages.dashboard.QueuedHint'),
            value: stats.queued,
            color: 'info',
          },
          {
            id: 'extracting',
            label: $_('pages.dashboard.Extracting'),
            hint: $_('pages.dashboard.ExtractingHint'),
            value: stats.extracting,
            color: 'primary',
          },
          {
            id: 'extracted',
            label: $_('pages.dashboard.Extracted'),
            hint: $_('pages.dashboard.ExtractedHint'),
            value: stats.extracted,
            color: 'success',
          },
          {
            id: 'imported',
            label: $_('pages.dashboard.Imported'),
            hint: $_('pages.dashboard.ImportedHint'),
            value: stats.imported,
            color: 'success',
          },
          {
            id: 'failed',
            label: $_('pages.dashboard.Failed'),
            hint: $_('pages.dashboard.FailedHint'),
            value: stats.failed,
            color: 'danger',
          },
          {
            id: 'deleted',
            label: $_('pages.dashboard.Deleted'),
            hint: $_('pages.dashboard.DeletedHint'),
            value: stats.deleted,
            color: 'dark',
          },
        ]
      : [],
  )

  type SideRow = {
    id: string
    label: string
    hint: string
    value: string
    fail?: number
    full?: boolean
  }

  function stackValue(buf?: BufferStat): string {
    return `${buf?.len ?? 0} / ${buf?.cap ?? 0}`
  }

  function stackFull(buf?: BufferStat): boolean {
    return (buf?.cap ?? 0) > 0 && (buf?.len ?? 0) >= (buf?.cap ?? 0)
  }

  function stackRow(
    id: string,
    label: string,
    hint: string,
    buf?: BufferStat,
  ): SideRow {
    return { id, label, hint, value: stackValue(buf), full: stackFull(buf) }
  }

  const side = $derived.by((): SideRow[] => {
    if (!stats) return []

    const rows: SideRow[] = [
      {
        id: 'finished',
        label: $_('pages.dashboard.Finished'),
        hint: $_('pages.dashboard.FinishedHint'),
        value: String(stats.finished ?? 0),
      },
      {
        id: 'retries',
        label: $_('pages.dashboard.Retries'),
        hint: '',
        value: String(stats.retries),
      },
    ]
    if (stats.starrs) {
      rows.push({
        id: 'starrs',
        label: $_('pages.dashboard.Starrs'),
        hint: $_('pages.dashboard.StarrsHint'),
        value: String(stats.starrs),
      })
    }
    if (stats.folders) {
      rows.push({
        id: 'folders',
        label: $_('pages.dashboard.Folders'),
        hint: $_('pages.dashboard.FoldersHint'),
        value: String(stats.folders),
      })
    }
    if (stats.webhooks) {
      rows.push({
        id: 'webhooks',
        label: $_('pages.dashboard.Webhooks'),
        hint: $_('pages.dashboard.WebhooksHint'),
        value: String(stats.hookOK ?? 0),
        fail: stats.hookFail ?? 0,
      })
    }
    if (stats.cmdhooks) {
      rows.push({
        id: 'cmdhooks',
        label: $_('pages.dashboard.Cmdhooks'),
        hint: $_('pages.dashboard.CmdhooksHint'),
        value: String(stats.cmdOK ?? 0),
        fail: stats.cmdFail ?? 0,
      })
    }
    if (stats.folders) {
      rows.push(
        stackRow(
          'stack-fs',
          $_('pages.dashboard.StackFS'),
          $_('pages.dashboard.StackFSHint'),
          stats.stackFS,
        ),
        stackRow(
          'stack-folder',
          $_('pages.dashboard.StackFolder'),
          $_('pages.dashboard.StackFolderHint'),
          stats.stackFolder,
        ),
      )
    }
    if (stats.starrs) {
      rows.push(
        stackRow(
          'stack-xtractr',
          $_('pages.dashboard.StackXtractr'),
          $_('pages.dashboard.StackXtractrHint'),
          stats.stackXtractr,
        ),
      )
    }
    if (stats.webhooks || stats.cmdhooks) {
      rows.push(
        stackRow(
          'stack-hook',
          $_('pages.dashboard.StackHook'),
          $_('pages.dashboard.StackHookHint'),
          stats.stackHook,
        ),
      )
    }
    rows.push(
      stackRow(
        'stack-del',
        $_('pages.dashboard.StackDel'),
        $_('pages.dashboard.StackDelHint'),
        stats.stackDel,
      ),
      stackRow(
        'stack-task',
        $_('pages.dashboard.StackTask'),
        $_('pages.dashboard.StackTaskHint'),
        stats.stackTask,
      ),
    )
    return rows
  })

  const starrQueues = $derived(stats?.starrQueues ?? [])

  function showBar(item: QueueItem): boolean {
    return (
      item.status === 'extracting' &&
      !!(
        item.percent ||
        item.total ||
        item.compressed ||
        item.wrote ||
        item.read ||
        item.archive ||
        item.archives
      )
    )
  }

  const dueKeys: Record<string, string> = {
    start: 'pages.dashboard.DueStart',
    retry: 'pages.dashboard.DueRetry',
    cleanup: 'pages.dashboard.DueCleanup',
    history: 'pages.dashboard.DueHistory',
  }

  function dueLabel(item: QueueItem, clock: number): string {
    if (isZeroTime(item.due) || !item.dueKind) return ''
    const key = dueKeys[item.dueKind]
    if (!key) return ''
    const remain = remainCompact(item.due, clock)
    if (!remain) return $_('pages.dashboard.DueSoon')
    return $_(key, { values: { remain } })
  }

  function extractingBytes(item: QueueItem): string {
    const max = item.total || item.compressed || 0
    if (max || item.percent) return progressCaption(item)
    return ''
  }

  function extractingSpeeds(item: QueueItem): string {
    const parts: string[] = []
    if (item.avgSpeedBps) {
      parts.push(
        $_('pages.dashboard.SpeedAvg', {
          values: { speed: bytes(item.avgSpeedBps) },
        }),
      )
    }
    if (item.speedBps) {
      parts.push(
        $_('pages.dashboard.SpeedNow', {
          values: { speed: bytes(item.speedBps) },
        }),
      )
    }
    return parts.join(' · ')
  }

  function extractingEta(item: QueueItem, clock: number): string {
    if (isZeroTime(item.eta)) return ''
    const remain = remainCompact(item.eta, clock)
    return remain
      ? $_('pages.dashboard.ETA', { values: { remain } })
      : $_('pages.dashboard.DueSoon')
  }

  function extractingCaption(item: QueueItem, clock: number): string {
    return [extractingBytes(item), extractingSpeeds(item), extractingEta(item, clock)]
      .filter(Boolean)
      .join(' · ')
  }

  function archiveLabel(item: QueueItem): string {
    if (item.archives) {
      const n = (item.extracted ?? 0) + 1
      const of = $_('pages.dashboard.ArchiveOf', {
        values: { n, total: item.archives },
      })
      return item.archive ? `${of} · ${item.archive}` : of
    }
    return item.archive || ''
  }

  async function retry(item: QueueItem) {
    busy[item.id] = true
    const res = await api.post('queue/retry', { id: item.id })
    busy[item.id] = false
    if (res.ok)
      success($_('pages.dashboard.Retrying', { values: { id: item.id } }))
    else failure(res.body?.error ?? 'retry failed')
  }

  function wantsCleanupWarning(item: QueueItem): boolean {
    return item.status.toLowerCase() === 'imported'
  }

  function forget(item: QueueItem) {
    if (wantsCleanupWarning(item)) {
      pendingForget = item
      return
    }
    void runForget(item)
  }

  function cancelForget() {
    pendingForget = null
  }

  function confirmForget() {
    const item = pendingForget
    pendingForget = null
    if (item) void runForget(item)
  }

  async function runForget(item: QueueItem) {
    busy[item.id] = true
    const res = await api.post('queue/forget', { id: item.id })
    busy[item.id] = false
    if (res.ok)
      success($_('pages.dashboard.Forgotten', { values: { id: item.id } }))
    else failure(res.body?.error ?? 'forget failed')
  }

  onMount(() => {
    queueColumnWidths = loadColumnWidths(
      queueColumnStorageKey,
      queueColumnDefaults,
    )
    window.addEventListener('pointermove', moveQueueResize)
    window.addEventListener('pointerup', finishQueueResize)
    window.addEventListener('mousemove', moveQueueResize)
    window.addEventListener('mouseup', finishQueueResize)
    const topics: LiveTopic[] = []
    if (canQueue) {
      topics.push('queue')
      topics.push('progress')
    }
    if (canHistory) topics.push('history')
    if (topics.length) live.subscribe(topics)
    void live.restSnapshot()
    ageTimer = setInterval(() => {
      now = Date.now()
    }, 1000)
    return () => {
      if (topics.length) live.unsubscribe(topics)
      window.removeEventListener('pointermove', moveQueueResize)
      window.removeEventListener('pointerup', finishQueueResize)
      window.removeEventListener('mousemove', moveQueueResize)
      window.removeEventListener('mouseup', finishQueueResize)
      document.body.classList.remove('is-resizing-columns')
    }
  })
  onDestroy(() => {
    if (ageTimer) clearInterval(ageTimer)
  })
</script>

<div class="dashboard-page">
  <section class="dashboard-hero">
  {#if canStats && stats}
    <div class="dashboard-summary">
      <div class="dashboard-stat-column">
        <div class="dashboard-stat-grid" aria-label="Status totals">
          {#each cards as c (c.id)}
            <Card
              id="{uid}-{c.id}"
              class="dashboard-panel stat-card stat-card-{c.id} text-center"
            >
              <CardBody>
                <div class="text-muted text-uppercase">{c.label}</div>
                <div class="stat-value text-{c.color}">{c.value}</div>
              </CardBody>
            </Card>
            <Tooltip target="{uid}-{c.id}" placement="top">{c.hint}</Tooltip>
          {/each}
        </div>
      </div>
      <div class="dashboard-meta-rail" aria-label="Status details">
        {#each side as row (row.id)}
          <span class="pill">
            <strong id="{uid}-{row.id}">{row.label}</strong>
            <span class:full={row.full} class:text-danger={row.full}>
              {#if row.fail === undefined}
                {row.value}
              {:else}
                {row.value} /
                <span class:text-danger={row.fail > 0}>{row.fail}</span>
              {/if}
            </span>
          </span>
          {#if row.hint}
            <Tooltip target="{uid}-{row.id}" placement="top">{row.hint}</Tooltip>
          {/if}
        {/each}
      </div>
    </div>
  {/if}
  </section>

  {#if (canStats || canQueue) && loading}
    <Spinner color="primary" />
  {:else}
  {#if canStats && starrQueues.length}
    <Card class="dashboard-panel mb-3 starr-queues-panel">
      <CardBody class="py-2 px-2">
        <CardTitle class="mb-2" id="{uid}-starr-queues"
          >{$_('pages.dashboard.StarrQueues')}</CardTitle
        >
        <Tooltip target="{uid}-starr-queues" placement="top"
          >{$_('pages.dashboard.StarrQueuesHint')}</Tooltip
        >
        <Table size="sm" striped borderless class="stat-side-table mb-0">
          <thead>
            <tr>
              <th scope="col">{$_('pages.dashboard.App')}</th>
              <th id="{uid}-starrq-h-total" class="text-end" scope="col"
                >{$_('pages.dashboard.StarrQueueTotal')}</th
              >
              <th id="{uid}-starrq-h-complete" class="text-end" scope="col"
                >{$_('pages.dashboard.StarrQueueComplete')}</th
              >
              <th id="{uid}-starrq-h-match" class="text-end" scope="col"
                >{$_('pages.dashboard.StarrQueueMatch')}</th
              >
              <th id="{uid}-starrq-h-issues" class="text-end" scope="col"
                >{$_('pages.dashboard.StarrQueueIssues')}</th
              >
              <th id="{uid}-starrq-h-dl" class="text-end" scope="col"
                >{$_('pages.dashboard.StarrQueueDownloading')}</th
              >
            </tr>
          </thead>
          <tbody>
            {#each starrQueues as row, i (`${row.app}-${row.url}-${i}`)}
              <tr>
                <th id="{uid}-starrq-{i}" scope="row">{row.name}</th>
                <td class="text-end" class:text-danger={!!row.error}>
                  {#if row.queued !== row.retrieved}
                    {row.queued} / {row.retrieved}
                  {:else}
                    {row.queued}
                  {/if}
                </td>
                <td class="text-end">{row.complete ?? 0}</td>
                <td class="text-end">{row.match ?? 0}</td>
                <td class="text-end" class:text-danger={(row.issues ?? 0) > 0}
                  >{row.issues ?? 0}</td
                >
                <td class="text-end">{row.downloading ?? 0}</td>
              </tr>
              <Tooltip target="{uid}-starrq-{i}" placement="top">
                {row.error || row.url || $_('pages.dashboard.StarrQueuesHint')}
              </Tooltip>
            {/each}
          </tbody>
        </Table>
        <Tooltip target="{uid}-starrq-h-total" placement="top"
          >{$_('pages.dashboard.StarrQueueTotalHint')}</Tooltip
        >
        <Tooltip target="{uid}-starrq-h-complete" placement="top"
          >{$_('pages.dashboard.StarrQueueCompleteHint')}</Tooltip
        >
        <Tooltip target="{uid}-starrq-h-match" placement="top"
          >{$_('pages.dashboard.StarrQueueMatchHint')}</Tooltip
        >
        <Tooltip target="{uid}-starrq-h-issues" placement="top"
          >{$_('pages.dashboard.StarrQueueIssuesHint')}</Tooltip
        >
        <Tooltip target="{uid}-starrq-h-dl" placement="top"
          >{$_('pages.dashboard.StarrQueueDownloadingHint')}</Tooltip
        >
      </CardBody>
    </Card>
  {/if}

  {#if canQueue}
    <Card class="dashboard-panel activity-panel">
      <CardBody>
        <Row class="align-items-center mb-2">
          <Col>
            <CardTitle class="mb-0">{$_('pages.dashboard.ActiveQueue')} ({trackedCount})</CardTitle>
            <div class="subtle small mt-1">{$_('pages.dashboard.TrackedHint')}</div>
          </Col>
          <Col xs="auto" class="d-flex align-items-center gap-2">
            <Button
              color="secondary"
              outline
              size="sm"
              type="button"
              onclick={resetQueueColumns}
              >{$_('buttons.ResetColumns')}</Button
            >
          </Col>
        </Row>
        {#if queue.length === 0}
          <p class="text-muted mb-0">{$_('phrases.NothingQueued')}</p>
        {:else}
          <Table responsive hover size="sm" class="align-middle queue-table" style={`width: ${queueTableWidth}`}>
            <colgroup>
              <col style={`width: ${colStyle(queueColumnWidths.item)}`} />
              <col style={`width: ${colStyle(queueColumnWidths.status)}`} />
              <col style={`width: ${colStyle(queueColumnWidths.progress)}`} />
              {#if queueShowsDelete}
                <col style={`width: ${colStyle(queueColumnWidths.deletes)}`} />
              {/if}
              <col style={`width: ${colStyle(queueColumnWidths.updated)}`} />
              <col style={`width: ${colStyle(queueColumnWidths.path)}`} />
              <col style={`width: ${colStyle(queueColumnWidths.actions)}`} />
            </colgroup>
            <thead>
              <tr>
                <th data-column-key="item">
                  {$_('pages.dashboard.Item')}
                  <button
                    type="button"
                    class="column-resizer"
                    aria-label="Resize item column"
                    title="Resize item column"
                    onpointerdown={(event) => startQueueResize(event, 'item')}
                    onkeydown={(event) => nudgeQueueResize(event, 'item')}
                  ></button>
                </th>
                <th data-column-key="status">
                  {$_('pages.dashboard.Status')}
                  <button
                    type="button"
                    class="column-resizer"
                    aria-label="Resize status column"
                    title="Resize status column"
                    onpointerdown={(event) => startQueueResize(event, 'status')}
                    onkeydown={(event) => nudgeQueueResize(event, 'status')}
                  ></button>
                </th>
                <th class="queue-progress" data-column-key="progress">
                  {$_('pages.dashboard.Progress')}
                  <button
                    type="button"
                    class="column-resizer"
                    aria-label="Resize progress column"
                    title="Resize progress column"
                    onpointerdown={(event) => startQueueResize(event, 'progress')}
                    onkeydown={(event) => nudgeQueueResize(event, 'progress')}
                  ></button>
                </th>
                {#if queueShowsDelete}
                  <th data-column-key="deletes">
                    {$_('phrases.DeletesIn')}
                    <button
                      type="button"
                      class="column-resizer"
                      aria-label="Resize deletes in column"
                      title="Resize deletes in column"
                      onpointerdown={(event) => startQueueResize(event, 'deletes')}
                      onkeydown={(event) => nudgeQueueResize(event, 'deletes')}
                    ></button>
                  </th>
                {/if}
                <th data-column-key="updated">
                  {$_('pages.dashboard.Updated')}
                  <button
                    type="button"
                    class="column-resizer"
                    aria-label="Resize updated column"
                    title="Resize updated column"
                    onpointerdown={(event) => startQueueResize(event, 'updated')}
                    onkeydown={(event) => nudgeQueueResize(event, 'updated')}
                  ></button>
                </th>
                <th data-column-key="path">
                  {$_('pages.logs.Path')}
                  <button
                    type="button"
                    class="column-resizer"
                    aria-label="Resize path column"
                    title="Resize path column"
                    onpointerdown={(event) => startQueueResize(event, 'path')}
                    onkeydown={(event) => nudgeQueueResize(event, 'path')}
                  ></button>
                </th>
                <th data-column-key="actions" class="text-end">
                  {$_('pages.dashboard.Actions')}
                </th>
              </tr>
            </thead>
            <tbody>
              {#each queue as item (item.id)}
                <tr
                  class="items-row"
                  class:is-selected={selectedQueueId === item.id}
                  tabindex="0"
                  role="button"
                  aria-pressed={selectedQueueId === item.id}
                  aria-label={itemTitle(item)}
                  onclick={() => toggleQueueSelect(item.id)}
                  onkeydown={(event) => {
                    if (event.target !== event.currentTarget) return
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault()
                      toggleQueueSelect(item.id)
                    }
                  }}
                >
                  <td data-label={$_('pages.dashboard.Item')} class="items-cell-item">
                    <div class="item-title"><strong>{itemTitle(item)}</strong></div>
                    <div class="item-app text-muted">{item.app}</div>
                    {#if item.retries}
                      <div class="item-note text-muted">{$_('pages.dashboard.Retries')}: {item.retries}</div>
                    {/if}
                    {#if itemReason(item)}
                    <div class="item-note text-muted"><strong>Reason</strong> {itemReason(item)}</div>
                    {/if}
                    {#if item.error}
                      <div class="item-note text-danger"><strong>Error</strong> {item.error}</div>
                    {/if}
                  </td>
                  <td data-label={$_('pages.dashboard.Status')}>
                    <Badge color={statusColor(item.status)}
                      >{$_(statusPhrase(item.status))}</Badge
                    >
                  </td>
                  <td data-label={$_('pages.dashboard.Progress')} class="queue-progress">
                    <div class="queue-progress-inner">
                      {#if showBar(item)}
                        {@const cap = extractingCaption(item, now)}
                        {@const bytesLine = extractingBytes(item)}
                        {@const speeds = extractingSpeeds(item)}
                        {@const eta = extractingEta(item, now)}
                        {@const arch = archiveLabel(item)}
                        <div class="progress mb-1">
                          <div
                            class="progress-bar"
                            style="width: {Math.min(item.percent ?? 0, 100)}%"
                          ></div>
                        </div>
                        {#if bytesLine}
                          <div class="queue-progress-caption" title={cap}>
                            {bytesLine}
                          </div>
                        {/if}
                        {#if speeds}
                          <div class="queue-progress-caption text-muted">
                            {speeds}
                          </div>
                        {/if}
                        {#if eta}
                          <div class="queue-progress-caption text-muted">
                            {eta}
                          </div>
                        {/if}
                        {#if arch}
                          <div
                            class="queue-progress-archive text-muted"
                            title={item.archive || arch}
                          >
                            {arch}
                          </div>
                        {/if}
                      {:else if item.status === 'waiting' && item.app === 'Folder'}
                        {@const due = dueLabel(item, now)}
                        <div class="queue-progress-caption">
                          {$_('pages.dashboard.LastWrite')}
                          {relTime(item.updated, now)}
                        </div>
                        {#if due}
                          <div
                            class="queue-progress-caption text-muted"
                            title={item.due}
                          >
                            {due}
                          </div>
                        {/if}
                      {:else}
                        {@const cap = progressCaption(item)}
                        {@const due = dueLabel(item, now)}
                        {#if cap}
                          <div
                            class="queue-progress-caption text-muted"
                            title={item.progress || ''}
                          >
                            {cap}
                          </div>
                        {/if}
                        {#if due}
                          <div
                            class="queue-progress-caption text-muted"
                            title={item.due}
                          >
                            {due}
                          </div>
                        {:else if !cap}
                          <div class="queue-progress-caption text-muted">
                            {$_('phrases.Empty')}
                          </div>
                        {/if}
                      {/if}
                    </div>
                  </td>
                  {#if queueShowsDelete}
                    <td data-label={$_('phrases.DeletesIn')} class="text-nowrap">
                      {item.deleteAt ? deleteRemaining(item.deleteAt, now) || $_('phrases.Empty') : $_('phrases.Empty')}
                    </td>
                  {/if}
                  <td data-label={$_('pages.dashboard.Updated')} class="text-nowrap" title={item.updated}>
                    {relTime(item.updated, now)}
                  </td>
                  <td data-label={$_('pages.logs.Path')} class="items-cell-path">
                    <code class="path-cell">{itemPath(item)}</code>
                  </td>
                  <td data-label={$_('pages.dashboard.Actions')} class="text-end text-nowrap">
                    <ButtonGroup size="sm">
                      <Button
                        type="button"
                        color="secondary"
                        outline
                        onclick={(e) => {
                          e.stopPropagation()
                          selectedQueueId = item.id
                        }}
                        >{$_('phrases.Details')}</Button
                      >
                      {#if canWrite && (item.status === 'extractfailed' || TERMINAL.includes(item.status))}
                          {#if item.status === 'extractfailed'}
                            <Button
                              color="primary"
                              outline
                              disabled={busy[item.id]}
                              onclick={(e) => {
                                e.stopPropagation()
                                retry(item)
                              }}
                              >{$_('buttons.Retry')}</Button
                            >
                          {/if}
                          {#if TERMINAL.includes(item.status)}
                            <Button
                              type="button"
                              color="secondary"
                              outline
                              disabled={busy[item.id]}
                              onclick={(e) => {
                                e.stopPropagation()
                                forget(item)
                              }}
                              >{$_('buttons.Forget')}</Button
                            >
                          {/if}
                      {/if}
                    </ButtonGroup>
                  </td>
                </tr>
              {/each}
            </tbody>
          </Table>
        {/if}
      </CardBody>
    </Card>
    {#if selectedQueueItem}
      <div class="mt-3">
        <TaskDetails
          item={selectedQueueItem}
          {now}
          onclose={() => (selectedQueueId = null)}
        />
      </div>
    {/if}
  {/if}
{/if}

{#if canHistory}
  <div class="mt-4 history-section">
    <History {now} />
  </div>
{/if}
</div>

<Modal isOpen={pendingForget !== null} toggle={cancelForget}>
  <ModalHeader toggle={cancelForget}
    >{$_('phrases.ForgetImportedTitle')}</ModalHeader
  >
  <ModalBody>{$_('phrases.ForgetImportedConfirm')}</ModalBody>
  <ModalFooter>
    <Button color="secondary" type="button" onclick={cancelForget}
      >{$_('buttons.Cancel')}</Button
    >
    <Button color="danger" type="button" onclick={confirmForget}
      >{$_('buttons.Forget')}</Button
    >
  </ModalFooter>
</Modal>
