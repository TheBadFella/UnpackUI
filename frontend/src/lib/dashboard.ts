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
