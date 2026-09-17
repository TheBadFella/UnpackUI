export type ColumnDefaults<K extends string> = Record<K, number>

const minimumColumnWidth = 64

export function loadColumnWidths<K extends string>(
  storageKey: string,
  defaults: ColumnDefaults<K>,
): ColumnDefaults<K> {
  const output = { ...defaults }

  if (typeof localStorage === 'undefined') return output

  try {
    const saved = JSON.parse(localStorage.getItem(storageKey) ?? '') as Record<
      string,
      unknown
    >

    for (const key of Object.keys(defaults) as K[]) {
      const value = Number(saved?.[key])
      if (Number.isFinite(value) && value >= minimumColumnWidth) {
        output[key] = Math.round(value)
      }
    }
  } catch {
    // Invalid or unavailable browser storage should never block the dashboard.
  }

  return output
}

export function saveColumnWidths<K extends string>(
  storageKey: string,
  widths: ColumnDefaults<K>,
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
  defaults: ColumnDefaults<K>,
): ColumnDefaults<K> {
  if (typeof localStorage !== 'undefined') {
    try {
      localStorage.removeItem(storageKey)
    } catch {
      // Keep the in-memory reset even when storage is unavailable.
    }
  }

  return { ...defaults }
}

export function clampColumnWidth(width: number): number {
  return Math.max(minimumColumnWidth, Math.round(width))
}
