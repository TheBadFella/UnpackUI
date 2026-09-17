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
    clampColumnWidth,
    loadColumnWidths,
    resetColumnWidths,
    saveColumnWidths,
    colStyle,
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
    | 'app'
    | 'status'
    | 'files'
    | 'size'
    | 'retries'
    | 'finished'
    | 'actions'
  const historyColumnDefaults: Record<HistoryColumn, string> = {
    app: '13%',
    status: '13%',
    files: '8%',
    size: '11%',
    retries: '8%',
    finished: '27%',
    actions: '20%',
  }
  const historyColumnMins: Record<HistoryColumn, number> = {
    app: 70,
    status: 70,
    files: 50,
    size: 70,
    retries: 50,
    finished: 110,
    actions: 80,
  }
  const historyColumnStorageKey = 'unpackerr.dashboard.history-columns.v1'
  let historyColumnWidths = $state<Record<HistoryColumn, number | string>>({ ...historyColumnDefaults })
  let tableWidth = $state<number | null>(null)
  let historyResize = $state<{
    key: HistoryColumn
    startX: number
    startWidths: Record<HistoryColumn, number>
    containerWidth: number
    handle: HTMLElement
  } | null>(null)

  function resetHistoryColumns() {
    tableWidth = null
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
    const keys: HistoryColumn[] = ['app', 'status', 'files', 'size', 'retries', 'finished']
    if (canWrite) keys.push('actions')

    const tableEl = heading.closest('table')
    const containerWidth = tableEl?.parentElement?.clientWidth ?? tableEl?.getBoundingClientRect().width ?? 0

    const startWidths: Record<HistoryColumn, number> = {} as any
    ths.forEach((th, idx) => {
      const colKey = keys[idx]
      if (colKey) {
        startWidths[colKey] = Math.round(th.getBoundingClientRect().width)
      }
    })

    historyResize = {
      key,
      startX: event.clientX,
      startWidths,
      containerWidth,
      handle,
    }
    document.body.classList.add('is-resizing-columns')
  }

  function moveHistoryResize(event: PointerEvent | MouseEvent) {
    if (!historyResize) return
    const delta = event.clientX - historyResize.startX
    const flexKey: HistoryColumn = historyResize.key === 'finished' ? 'status' : 'finished'

    const minW = historyColumnMins[historyResize.key]
    const newTargetW = Math.max(minW, Math.round(historyResize.startWidths[historyResize.key] + delta))
    const actualDelta = newTargetW - historyResize.startWidths[historyResize.key]

    const flexMinW = historyColumnMins[flexKey]
    const newFlexW = Math.max(flexMinW, Math.round(historyResize.startWidths[flexKey] - actualDelta))
    const absorbedDelta = historyResize.startWidths[flexKey] - newFlexW

    const unabsorbed = actualDelta - absorbedDelta

    const updated: Record<HistoryColumn, number | string> = { ...historyResize.startWidths }
    updated[historyResize.key] = newTargetW
    updated[flexKey] = newFlexW
    historyColumnWidths = updated

    if (unabsorbed > 0) {
      tableWidth = Math.round(historyResize.containerWidth + unabsorbed)
    } else {
      tableWidth = null
    }
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
    const flexKey: HistoryColumn = key === 'finished' ? 'status' : 'finished'
    const curVal = typeof historyColumnWidths[key] === 'number' ? (historyColumnWidths[key] as number) : 120
    const flexVal = typeof historyColumnWidths[flexKey] === 'number' ? (historyColumnWidths[flexKey] as number) : 300
    const newTarget = Math.max(historyColumnMins[key], curVal + delta)
    const newFlex = Math.max(historyColumnMins[flexKey], flexVal - (newTarget - curVal))
    const updated: Record<HistoryColumn, number | string> = { ...historyColumnWidths }
    updated[key] = newTarget
    updated[flexKey] = newFlex
    historyColumnWidths = updated
    saveColumnWidths(historyColumnStorageKey, historyColumnWidths)
  }

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
          (r.id + r.app + r.status)
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
        <CardTitle class="mb-0">{$_('pages.history.Title')}</CardTitle>
      </Col>
      <Col xs="auto">
        <Input
          id="history-filter"
          name="history-filter"
          type="text"
          bsSize="sm"
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
      <Table responsive hover size="sm" class="align-middle history-table" style={tableWidth ? `width: ${tableWidth}px` : 'width: 100%'}>
        <colgroup>
          <col style={`width: ${colStyle(historyColumnWidths.app)}`} />
          <col style={`width: ${colStyle(historyColumnWidths.status)}`} />
          <col style={`width: ${colStyle(historyColumnWidths.files)}`} />
          <col style={`width: ${colStyle(historyColumnWidths.size)}`} />
          <col style={`width: ${colStyle(historyColumnWidths.retries)}`} />
          <col style={`width: ${colStyle(historyColumnWidths.finished)}`} />
          {#if canWrite}<col style={`width: ${colStyle(historyColumnWidths.actions)}`} />{/if}
        </colgroup>
        <thead>
          <tr>
            <th id="hist-app" scope="col">
              {$_('pages.history.App')}
              <button type="button" class="column-resizer" aria-label="Resize app column"
                onpointerdown={(event) => startHistoryResize(event, 'app')}
                onkeydown={(event) => nudgeHistoryResize(event, 'app')}></button>
            </th>
            <th id="hist-status" scope="col">
              {$_('pages.history.Status')}
              <button type="button" class="column-resizer" aria-label="Resize status column"
                onpointerdown={(event) => startHistoryResize(event, 'status')}
                onkeydown={(event) => nudgeHistoryResize(event, 'status')}></button>
            </th>
            <th id="hist-files" class="text-end" scope="col">
              {$_('pages.history.Files')}
              <button type="button" class="column-resizer" aria-label="Resize files column"
                onpointerdown={(event) => startHistoryResize(event, 'files')}
                onkeydown={(event) => nudgeHistoryResize(event, 'files')}></button>
            </th>
            <th id="hist-size" class="text-end" scope="col">
              {$_('pages.history.Size')}
              <button type="button" class="column-resizer" aria-label="Resize size column"
                onpointerdown={(event) => startHistoryResize(event, 'size')}
                onkeydown={(event) => nudgeHistoryResize(event, 'size')}></button>
            </th>
            <th id="hist-retries" class="text-end" scope="col">
              {$_('pages.history.Retries')}
              <button type="button" class="column-resizer" aria-label="Resize retries column"
                onpointerdown={(event) => startHistoryResize(event, 'retries')}
                onkeydown={(event) => nudgeHistoryResize(event, 'retries')}></button>
            </th>
            <th id="hist-finished" scope="col">
              {$_('pages.history.Finished')}
              {#if canWrite}
                <button type="button" class="column-resizer" aria-label="Resize finished column"
                  onpointerdown={(event) => startHistoryResize(event, 'finished')}
                  onkeydown={(event) => nudgeHistoryResize(event, 'finished')}></button>
              {/if}
            </th>
            {#if canWrite}
              <th id="hist-actions" class="text-end" scope="col">
                {$_('pages.history.Actions')}
              </th>
            {/if}
          </tr>
        </thead>
        {#each shown as row (row.id)}
          <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
          <tbody
            class="stack-item items-row"
            class:is-selected={selectedHistoryId === row.id}
            onclick={() => toggleHistorySelect(row.id)}
          >
            <tr>
              <td data-label={$_('pages.history.App')} headers="hist-app">{row.app}</td>
              <td data-label={$_('pages.history.Status')} headers="hist-status"
                ><Badge color={statusColor(row.status)}
                  >{$_(statusPhrase(row.status))}</Badge
                ></td
              >
              <td data-label={$_('pages.history.Files')} class="text-end" headers="hist-files">{row.files || 0}</td>
              <td data-label={$_('pages.history.Size')} class="text-end text-nowrap" headers="hist-size"
                >{bytes(row.bytes)}</td
              >
              <td data-label={$_('pages.history.Retries')} class="text-end" headers="hist-retries">{row.retries}</td>
              <td
                data-label={$_('pages.history.Finished')}
                class="small text-nowrap"
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
              {#if canWrite}
                <td data-label={$_('pages.history.Actions')} class="text-end" headers="hist-actions">
                  <Button
                    size="sm"
                    color="secondary"
                    outline
                    disabled={busy[row.id]}
                    onclick={(e) => {
                      e.stopPropagation()
                      remove(row)
                    }}>{$_('buttons.Delete')}</Button
                  >
                </td>
              {/if}
            </tr>
            <tr class="stack-item-path">
              <td colspan={canWrite ? 7 : 6} headers="hist-app">
                <div class="small"><strong>{itemTitle(row)}</strong></div>
                <code class="wrap small">{itemPath(row)}</code>
                {#if itemReason(row)}
                  <div class="text-muted small">{itemReason(row)}</div>
                {/if}
                {#if row.error}<div class="text-danger small">
                    {row.error}
                  </div>{/if}
              </td>
            </tr>
          </tbody>
        {/each}
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
