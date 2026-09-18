<script lang="ts">
  import { onMount } from 'svelte'
  import {
    Badge,
    Button,
    ButtonGroup,
    Card,
    CardBody,
    CardTitle,
    Col,
    Input,
    Row,
    Modal,
    ModalBody,
    ModalFooter,
    ModalHeader,
    Spinner,
    Table,
  } from '@sveltestrap/sveltestrap'
  import { _ } from '../lib/i18n/Translate.svelte'
  import { api } from '../lib/api'
  import { has } from '../lib/auth.svelte'
  import { systemPerm } from '../lib/perms'
  import {
    statusColor,
    statusPhrase,
    bytes,
    dateTime,
    relTime,
    isZeroTime,
    isFinishedHistory,
  } from '../lib/format'
  import {
    deleteRemaining,
    itemPath,
    itemReason,
    itemTitle,
    visibleFiles,
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
  import type { HistoryRecord } from '../lib/types'
  import { live } from '../lib/socket.svelte'
  import TaskDetails from '../components/TaskDetails.svelte'

  let { now = Date.now() }: { now?: number } = $props()

  let filter = $state('')
  let busy = $state<Record<string, boolean>>({})
  let loaded = $state(false)
  let pendingClear = $state(false)
  let dismissedIds = $state<string[]>([])
  let selectedHistoryId = $state<string | null>(null)

  const dismissedStorageKey = 'unpackerr.dashboard.dismissed-history.v1'

  type HistoryColumn =
    | 'item'
    | 'status'
    | 'files'
    | 'size'
    | 'retries'
    | 'finished'
    | 'path'
    | 'actions'
  const historyColumnDefaults: Record<HistoryColumn, string> = {
    item: '25%',
    status: '12%',
    files: '7%',
    size: '10%',
    retries: '7%',
    finished: '16%',
    path: '15%',
    actions: '8%',
  }
  const historyColumnMins: Record<HistoryColumn, number> = {
    item: 180,
    status: 110,
    files: 50,
    size: 70,
    retries: 50,
    finished: 130,
    path: 160,
    actions: 150,
  }
  const historyColumnKeys: HistoryColumn[] = [
    'item',
    'status',
    'files',
    'size',
    'retries',
    'finished',
    'path',
    'actions',
  ]
  const historyColumnStorageKey = 'unpackerr.dashboard.history-columns.v1'
  let historyColumnWidths = $state<Record<HistoryColumn, number | string>>({ ...historyColumnDefaults })
  let historyResize = $state<{
    key: HistoryColumn
    startX: number
    startWidths: Record<HistoryColumn, number>
    handle: HTMLElement
  } | null>(null)

  function resetHistoryColumns() {
    historyColumnWidths = resetColumnWidths(
      historyColumnStorageKey,
      historyColumnDefaults,
    )
  }

  function startHistoryResize(event: PointerEvent, key: HistoryColumn) {
    event.preventDefault()
    const handle = event.currentTarget as HTMLElement
    try {
      handle.setPointerCapture(event.pointerId)
    } catch {}
    const heading = handle.parentElement
    if (!heading) return

    const thead = heading.parentElement
    const ths = thead ? (Array.from(thead.children) as HTMLElement[]) : []

    const startWidths: Record<HistoryColumn, number> = {} as any
    ths.forEach((th) => {
      const colKey = th.dataset.columnKey as HistoryColumn | undefined
      if (colKey && historyColumnKeys.includes(colKey)) {
        startWidths[colKey] = Math.round(th.getBoundingClientRect().width)
      }
    })

    historyResize = {
      key,
      startX: event.clientX,
      startWidths,
      handle,
    }
    document.body.classList.add('is-resizing-columns')
  }

  function moveHistoryResize(event: PointerEvent | MouseEvent) {
    if (!historyResize) return
    const delta = event.clientX - historyResize.startX
    historyColumnWidths = resizedColumnWidths(
      historyResize.startWidths,
      historyResize.key,
      delta,
      historyColumnMins,
    )
  }

  function finishHistoryResize(event: PointerEvent | MouseEvent) {
    if (!historyResize) return
    try {
      if ('pointerId' in event && historyResize.handle?.hasPointerCapture(event.pointerId)) {
        historyResize.handle.releasePointerCapture(event.pointerId)
      }
    } catch {}
    saveColumnWidths(historyColumnStorageKey, historyColumnWidths)
    historyResize = null
    document.body.classList.remove('is-resizing-columns')
  }

  function nudgeHistoryResize(event: KeyboardEvent, key: HistoryColumn) {
    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
    event.preventDefault()
    const delta = event.key === 'ArrowRight' ? 16 : -16
    const handle = event.currentTarget as HTMLElement
    const table = handle.closest('table')
    if (!table) return
    const measured: Record<HistoryColumn, number> = {} as any
    table.querySelectorAll('thead th').forEach((th) => {
      const colKey = (th as HTMLElement).dataset.columnKey as HistoryColumn | undefined
      if (colKey) measured[colKey] = Math.round((th as HTMLElement).getBoundingClientRect().width)
    })
    historyColumnWidths = resizedColumnWidths(measured, key, delta, historyColumnMins)
    saveColumnWidths(historyColumnStorageKey, historyColumnWidths)
  }

  const historyTableWidth = $derived(tableWidth(historyColumnWidths, historyColumnKeys))

  const canWrite = has(systemPerm('history', 'write'))
  const allRows = $derived(live.history.filter(isFinishedHistory))
  const selectedHistoryItem = $derived(
    allRows.find((row) => row.id === selectedHistoryId) ?? null
  )

  function toggleHistorySelect(id: string) {
    selectedHistoryId = selectedHistoryId === id ? null : id
  }
  const rows = $derived(
    allRows.filter((row) => !dismissedIds.includes(row.id)),
  )
  const hiddenCount = $derived(allRows.length - rows.length)
  const loading = $derived(!loaded && rows.length === 0)

  const shown = $derived(
    filter.trim()
      ? rows.filter((r) =>
          (r.id + r.app + r.status + itemTitle(r) + itemPath(r))
            .toLowerCase()
            .includes(filter.trim().toLowerCase()),
        )
      : rows,
  )

  async function refresh() {
    const before = live.history
    const res = await api.get<HistoryRecord[]>('history')
    if (res.ok && live.history === before) {
      live.history = (res.body ?? []).filter(isFinishedHistory)
    }
    loaded = true
  }

  async function remove(row: HistoryRecord) {
    busy[row.id] = true
    const res = await api.post('history/delete', { id: row.id })
    busy[row.id] = false
    if (res.ok) {
      dismissedIds = dismissedIds.filter((id) => id !== row.id)
      saveDismissedIds(dismissedIds)
      success($_('pages.history.DeletedRow'))
    } else failure(res.body?.error ?? 'delete failed')
  }

  function askClearAll() {
    pendingClear = true
  }

  function loadDismissedIds(): string[] {
    try {
      const raw = window.localStorage.getItem(dismissedStorageKey)
      const parsed: unknown = raw ? JSON.parse(raw) : []
      return Array.isArray(parsed)
        ? parsed.filter((value): value is string => typeof value === 'string')
        : []
    } catch {
      return []
    }
  }

  function saveDismissedIds(ids: string[]) {
    try {
      window.localStorage.setItem(dismissedStorageKey, JSON.stringify(ids))
    } catch {
      // Restricted storage should not break the dashboard.
    }
  }

  function hideCompleted() {
    dismissedIds = [...new Set([...dismissedIds, ...allRows.map((row) => row.id)])]
    saveDismissedIds(dismissedIds)
  }

  function showHidden() {
    dismissedIds = []
    saveDismissedIds(dismissedIds)
  }

  function cancelClear() {
    pendingClear = false
  }

  async function confirmClear() {
    pendingClear = false
    const res = await api.post('history/clear', {})
    if (res.ok) {
      dismissedIds = []
      saveDismissedIds(dismissedIds)
      success($_('pages.history.Cleared'))
    } else failure(res.body?.error ?? 'clear failed')
  }

  onMount(() => {
    dismissedIds = loadDismissedIds()
    historyColumnWidths = loadColumnWidths(
      historyColumnStorageKey,
      historyColumnDefaults,
    )
    window.addEventListener('pointermove', moveHistoryResize)
    window.addEventListener('pointerup', finishHistoryResize)
    window.addEventListener('mousemove', moveHistoryResize)
    window.addEventListener('mouseup', finishHistoryResize)
    void refresh()

    return () => {
      window.removeEventListener('pointermove', moveHistoryResize)
      window.removeEventListener('pointerup', finishHistoryResize)
      window.removeEventListener('mousemove', moveHistoryResize)
      window.removeEventListener('mouseup', finishHistoryResize)
      document.body.classList.remove('is-resizing-columns')
    }
  })
</script>

<Card class="dashboard-panel history-panel">
  <CardBody>
    <Row class="align-items-center mb-2 g-2">
      <Col>
        <CardTitle class="mb-0">{$_('pages.history.Title')} ({allRows.length})</CardTitle>
        <div class="subtle small mt-1">{$_('pages.history.intro')}</div>
      </Col>
      <Col xs="auto">
        <Input
          id="history-filter"
          name="history-filter"
          type="text"
          aria-label={$_('phrases.FilterPlaceholder')}
          placeholder={$_('phrases.FilterPlaceholder')}
          bind:value={filter}
          style="width: 12rem"
        />
      </Col>
      <Col xs="auto">
        <ButtonGroup size="sm">
          <Button color="secondary" outline onclick={refresh}
            >{$_('buttons.Refresh')}</Button
          >
          <Button color="secondary" outline onclick={resetHistoryColumns}
            >{$_('buttons.ResetColumns')}</Button
          >
          <Button
            color="secondary"
            outline
            type="button"
            onclick={hideCompleted}
            disabled={allRows.length === 0}
            >{$_('buttons.HideCompleted')}</Button
          >
          {#if hiddenCount > 0}
            <Button color="secondary" outline type="button" onclick={showHidden}
              >{$_('buttons.ShowHidden')} ({hiddenCount})</Button
            >
          {/if}
          {#if canWrite}
            <Button
              color="danger"
              outline
              type="button"
              onclick={askClearAll}
              disabled={allRows.length === 0}
            >
              {$_('buttons.ClearAll')}
            </Button>
          {/if}
        </ButtonGroup>
      </Col>
    </Row>
    {#if loading}
      <Spinner color="primary" />
    {:else if shown.length === 0}
      <p class="text-muted mb-0">{$_('phrases.NoHistory')}</p>
    {:else}
      <Table responsive hover size="sm" class="align-middle history-table" style={`width: ${historyTableWidth}`}>
        <colgroup>
          <col style={`width: ${colStyle(historyColumnWidths.item)}`} />
          <col style={`width: ${colStyle(historyColumnWidths.status)}`} />
          <col style={`width: ${colStyle(historyColumnWidths.files)}`} />
          <col style={`width: ${colStyle(historyColumnWidths.size)}`} />
          <col style={`width: ${colStyle(historyColumnWidths.retries)}`} />
          <col style={`width: ${colStyle(historyColumnWidths.finished)}`} />
          <col style={`width: ${colStyle(historyColumnWidths.path)}`} />
          <col style={`width: ${colStyle(historyColumnWidths.actions)}`} />
        </colgroup>
        <thead>
          <tr>
            <th id="hist-item" data-column-key="item" scope="col">
              {$_('pages.history.Item')}
              <button type="button" class="column-resizer" aria-label="Resize item column"
                title="Resize item column"
                onpointerdown={(event) => startHistoryResize(event, 'item')}
                onkeydown={(event) => nudgeHistoryResize(event, 'item')}></button>
            </th>
            <th id="hist-status" data-column-key="status" scope="col">
              {$_('pages.history.Status')}
              <button type="button" class="column-resizer" aria-label="Resize status column"
                title="Resize status column"
                onpointerdown={(event) => startHistoryResize(event, 'status')}
                onkeydown={(event) => nudgeHistoryResize(event, 'status')}></button>
            </th>
            <th id="hist-files" data-column-key="files" class="text-end" scope="col">
              {$_('pages.history.Files')}
              <button type="button" class="column-resizer" aria-label="Resize files column"
                title="Resize files column"
                onpointerdown={(event) => startHistoryResize(event, 'files')}
                onkeydown={(event) => nudgeHistoryResize(event, 'files')}></button>
            </th>
            <th id="hist-size" data-column-key="size" class="text-end" scope="col">
              {$_('pages.history.Size')}
              <button type="button" class="column-resizer" aria-label="Resize size column"
                title="Resize size column"
                onpointerdown={(event) => startHistoryResize(event, 'size')}
                onkeydown={(event) => nudgeHistoryResize(event, 'size')}></button>
            </th>
            <th id="hist-retries" data-column-key="retries" class="text-end" scope="col">
              {$_('pages.history.Retries')}
              <button type="button" class="column-resizer" aria-label="Resize retries column"
                title="Resize retries column"
                onpointerdown={(event) => startHistoryResize(event, 'retries')}
                onkeydown={(event) => nudgeHistoryResize(event, 'retries')}></button>
            </th>
            <th id="hist-finished" data-column-key="finished" scope="col">
              {$_('pages.history.Finished')}
              <button type="button" class="column-resizer" aria-label="Resize finished column"
                title="Resize finished column"
                onpointerdown={(event) => startHistoryResize(event, 'finished')}
                onkeydown={(event) => nudgeHistoryResize(event, 'finished')}></button>
            </th>
            <th id="hist-path" data-column-key="path" scope="col">
              {$_('pages.logs.Path')}
              <button type="button" class="column-resizer" aria-label="Resize path column"
                title="Resize path column"
                onpointerdown={(event) => startHistoryResize(event, 'path')}
                onkeydown={(event) => nudgeHistoryResize(event, 'path')}></button>
            </th>
            <th id="hist-actions" data-column-key="actions" class="text-end" scope="col">
              {$_('pages.history.Actions')}
            </th>
          </tr>
        </thead>
        <tbody>
          {#each shown as row (row.id)}
            <tr
              class="items-row"
              class:is-selected={selectedHistoryId === row.id}
              tabindex="0"
              role="button"
              aria-pressed={selectedHistoryId === row.id}
              aria-label={itemTitle(row)}
              onclick={() => toggleHistorySelect(row.id)}
              onkeydown={(event) => {
                if (event.target !== event.currentTarget) return
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault()
                  toggleHistorySelect(row.id)
                }
              }}
            >
              <td data-label={$_('pages.history.Item')} headers="hist-item" class="items-cell-item">
                <div class="item-title"><strong>{itemTitle(row)}</strong></div>
                <div class="item-app text-muted">{row.app}</div>
                {#if row.retries}
                  <div class="item-note text-muted">{$_('pages.history.Retries')}: {row.retries}</div>
                {/if}
                {#if itemReason(row)}
                  <div class="item-note text-muted"><strong>Reason</strong> {itemReason(row)}</div>
                {/if}
                {#if row.error}
                  <div class="item-note text-danger"><strong>Error</strong> {row.error}</div>
                {/if}
              </td>
              <td data-label={$_('pages.history.Status')} headers="hist-status">
                <Badge color={statusColor(row.status)}
                  >{$_(statusPhrase(row.status))}</Badge
                >
              </td>
              <td data-label={$_('pages.history.Files')} class="text-end" headers="hist-files">{row.files || 0}</td>
              <td data-label={$_('pages.history.Size')} class="text-end text-nowrap" headers="hist-size"
                >{bytes(row.bytes)}</td
              >
              <td data-label={$_('pages.history.Retries')} class="text-end" headers="hist-retries">{row.retries}</td>
              <td
                data-label={$_('pages.history.Finished')}
                class="text-nowrap"
                headers="hist-finished"
                title={dateTime(row.finished)}
              >
                {isZeroTime(row.finished)
                  ? $_('phrases.Empty')
                  : relTime(row.finished, now)}
                {#if row.deleteAt}
                  <div class="text-muted">{$_('phrases.DeletesIn')} {deleteRemaining(row.deleteAt, now)}</div>
                {/if}
              </td>
              <td data-label={$_('pages.logs.Path')} headers="hist-path" class="items-cell-path">
                <code class="path-cell">{itemPath(row)}</code>
              </td>
              <td data-label={$_('pages.history.Actions')} class="text-end text-nowrap" headers="hist-actions">
                <ButtonGroup size="sm">
                  <Button
                    color="secondary"
                    outline
                    type="button"
                    onclick={(e) => {
                      e.stopPropagation()
                      selectedHistoryId = row.id
                    }}>{$_('phrases.Details')}</Button
                  >
                  {#if canWrite}
                  <Button
                    color="danger"
                    outline
                    type="button"
                    disabled={busy[row.id]}
                    onclick={(e) => {
                      e.stopPropagation()
                      remove(row)
                    }}>{$_('buttons.Delete')}</Button
                  >
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

{#if selectedHistoryItem}
  <div class="mt-3">
    <TaskDetails
      item={selectedHistoryItem}
      {now}
      onclose={() => (selectedHistoryId = null)}
    />
  </div>
{/if}

<Modal isOpen={pendingClear} toggle={cancelClear}>
  <ModalHeader toggle={cancelClear}
    >{$_('phrases.ClearHistoryTitle')}</ModalHeader
  >
  <ModalBody>{$_('phrases.ClearHistoryConfirm')}</ModalBody>
  <ModalFooter>
    <Button color="secondary" type="button" onclick={cancelClear}
      >{$_('buttons.Cancel')}</Button
    >
    <Button color="danger" type="button" onclick={confirmClear}
      >{$_('buttons.ClearAll')}</Button
    >
  </ModalFooter>
</Modal>
