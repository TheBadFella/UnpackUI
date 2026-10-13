import type { HistoryRecord, QueueItem } from './types'

const MARKER_FILE = /^_?unpackerred(?:\.|$)/i

export type DashboardRecord = QueueItem | HistoryRecord

export function itemTitle(item: DashboardRecord): string {
  const title = item.title?.trim()
  const normalizedTitle = title?.replace(/\\/g, '/').replace(/\/+$/, '')
  const normalizedPath = item.path.replace(/\\/g, '/').replace(/\/+$/, '')
  if (title && normalizedTitle !== normalizedPath) return title

  const label = normalizedPath.split('/').filter(Boolean).pop()
  return label || item.id
}

export function itemPath(item: DashboardRecord): string {
  return item.path || item.id
}

export function itemReason(item: DashboardRecord): string {
  return item.reason?.trim() || ''
}

export function deleteRemaining(
  deleteAt: string | undefined,
  now = Date.now(),
): string {
  if (!deleteAt) return ''
  const remaining = Date.parse(deleteAt) - now
  if (Number.isNaN(remaining)) return ''
  if (remaining <= 0) return 'due now'

  const seconds = Math.ceil(remaining / 1000)
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.ceil(seconds / 60)
  if (minutes < 60) return `${minutes}m`
  const hours = Math.ceil(minutes / 60)
  if (hours < 24) return `${hours}h`
  return `${Math.ceil(hours / 24)}d`
}

export function formatRate(bytesPerSecond: number | undefined): string {
  if (!bytesPerSecond || bytesPerSecond <= 0) return ''
  const units = ['B/s', 'KiB/s', 'MiB/s', 'GiB/s']
  let value = bytesPerSecond
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit++
  }

  return `${value.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`
}

export function formatEta(seconds: number | undefined): string {
  if (!seconds || seconds <= 0) return ''
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.ceil(seconds / 60)
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  const rest = minutes % 60
  return rest ? `${hours}h ${rest}m` : `${hours}h`
}

export function visibleFiles(files: string[] | undefined): string[] {
  return (files ?? []).filter((file) => !MARKER_FILE.test(file.split(/[\\/]/).pop() ?? ''))
}

export function statusHasExtractResult(status: string): boolean {
  switch (status.toLowerCase()) {
    case 'extracted':
    case 'extractednothing':
    case 'extractfailed':
    case 'imported':
    case 'deleting':
    case 'deleted':
    case 'deletefailed':
      return true
    default:
      return false
  }
}

export function shouldRefetchDetail(
  open: QueueItem | null,
  currentStatus: string | undefined,
  detailFilesReady: boolean,
): boolean {
  if (!open || detailFilesReady || !currentStatus) return false
  return statusHasExtractResult(currentStatus)
}

export function isDetailResponseValid(
  requestSeq: number,
  activeSeq: number,
  itemId: string,
  activeItemId: string | undefined,
): boolean {
  return requestSeq === activeSeq && Boolean(activeItemId) && itemId === activeItemId
}

export function mergeQueueDetail(
  pending: QueueItem | null,
  liveRow: QueueItem | HistoryRecord | null | undefined,
  detailFiles: QueueItem | null,
): QueueItem | null {
  if (!pending) return null
  const base = liveRow ?? pending
  const baseArchiveFiles = 'archiveFiles' in base ? base.archiveFiles : undefined
  const baseExtraFiles = 'extraFiles' in base ? base.extraFiles : undefined

  // Completed or history items have authoritative output paths and final file lists.
  // When an item has reached a finished or history state, its explicitly provided
  // file arrays and output paths (even if empty, e.g. extractednothing) are authoritative.
  // In-flight progress frames omit file lists entirely (undefined); in that case,
  // supplement with detailFiles.
  const isHistoryOrFinished =
    Boolean(liveRow) &&
    (('finished' in base && Boolean(base.finished)) || statusHasExtractResult(base.status))

  const newFiles = isHistoryOrFinished
    ? (Array.isArray(base.newFiles) ? base.newFiles : (detailFiles?.newFiles ?? base.newFiles))
    : (detailFiles?.newFiles && detailFiles.newFiles.length > 0
        ? detailFiles.newFiles
        : base.newFiles)

  const origFiles = isHistoryOrFinished
    ? (Array.isArray(base.origFiles) ? base.origFiles : (detailFiles?.origFiles ?? base.origFiles))
    : (detailFiles?.origFiles && detailFiles.origFiles.length > 0
        ? detailFiles.origFiles
        : base.origFiles)

  const archiveFiles = isHistoryOrFinished
    ? (Array.isArray(baseArchiveFiles) ? baseArchiveFiles : (detailFiles?.archiveFiles ?? baseArchiveFiles))
    : (detailFiles?.archiveFiles && detailFiles.archiveFiles.length > 0
        ? detailFiles.archiveFiles
        : baseArchiveFiles)

  const extraFiles = isHistoryOrFinished
    ? (Array.isArray(baseExtraFiles) ? baseExtraFiles : (detailFiles?.extraFiles ?? baseExtraFiles))
    : (detailFiles?.extraFiles && detailFiles.extraFiles.length > 0
        ? detailFiles.extraFiles
        : baseExtraFiles)

  const baseOutputPath = 'outputPath' in base ? base.outputPath : undefined
  const baseOutput = 'output' in base ? base.output : undefined

  const outputPath = isHistoryOrFinished
    ? (baseOutputPath !== undefined ? baseOutputPath : (detailFiles?.outputPath || ''))
    : (detailFiles?.outputPath || baseOutputPath || '')

  const output = isHistoryOrFinished
    ? (baseOutput !== undefined ? baseOutput : (detailFiles?.output || ''))
    : (detailFiles?.output || baseOutput || '')

  return {
    ...base,
    newFiles,
    origFiles,
    archiveFiles,
    extraFiles,
    outputPath,
    output,
    ids: base.ids ?? detailFiles?.ids,
  } as unknown as QueueItem
}
