import { expect, it, vi } from 'vitest'

it.each(['light', 'dark', ''])('uses dark mode with a saved %s preference and a light OS', async (saved) => {
  vi.resetModules()
  document.cookie = `un-theme=${saved}; path=/`
  document.documentElement.removeAttribute('data-bs-theme')
  const { theme } = await import('./theme.svelte')
  expect(theme.resolved).toBe('dark')
  expect(document.documentElement.dataset.bsTheme).toBe('dark')
  expect(document.documentElement.dataset.unTheme).toBe('dark')
})
