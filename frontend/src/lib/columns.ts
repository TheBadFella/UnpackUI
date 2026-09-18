export type ColumnWidths<K extends string> = Record<K, number | string>

const minimumColumnWidth = 50

export function loadColumnWidths<K extends string>(
  storageKey: string,
  defaults: ColumnWidths<K>,
): ColumnWidths<K> {
  const output = { ...defaults }

  if (typeof localStorage === 'undefined') return output

  try {
    const raw = localStorage.getItem(storageKey)
    if (!raw) return output
    const saved = JSON.parse(raw) as Record<string, unknown>

    for (const key of Object.keys(defaults) as K[]) {
      const val = saved?.[key]
      if (typeof val === 'string' && val.endsWith('%')) {
        output[key] = val
      } else {
        const num = Number(val)
        if (Number.isFinite(num) && num >= minimumColumnWidth) {
          output[key] = Math.round(num)
        }
      }
    }
  } catch {
    // Invalid or unavailable browser storage should never block the dashboard.
  }

  return output
}

export function saveColumnWidths<K extends string>(
  storageKey: string,
  widths: ColumnWidths<K>,
): void {
  if (typeof localStorage === 'undefined') return

  try {
    localStorage.setItem(storageKey, JSON.stringify(widths))
  } catch {
    // Private browsing and storage quotas are harmless; keep widths in memory.
  }
}

export function resetColumnWidths<K extends string>(
  storageKey: string,
  defaults: ColumnWidths<K>,
): ColumnWidths<K> {
  if (typeof localStorage !== 'undefined') {
    try {
      localStorage.removeItem(storageKey)
    } catch {
      // Keep the in-memory reset even when storage is unavailable.
    }
  }

  return { ...defaults }
}

export function colStyle(val: number | string | undefined, fallback = 'auto'): string {
  if (val === undefined) return fallback
  return typeof val === 'number' ? `${val}px` : val
}

export function resizedColumnWidths<K extends string>(
  widths: Record<K, number>,
  key: K,
  delta: number,
  minimums: Record<K, number>,
): Record<K, number> {
  return {
    ...widths,
    [key]: Math.max(minimums[key], Math.round(widths[key] + delta)),
  }
}

export function tableWidth<K extends string>(
  widths: ColumnWidths<K>,
  visible: K[],
): string {
  let total = 0
  for (const key of visible) {
    const width = widths[key]
    if (typeof width !== 'number') return '100%'
    total += width
  }

  return `max(100%, ${total}px)`
}
