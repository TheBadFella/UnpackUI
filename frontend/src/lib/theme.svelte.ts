// Bootstrap tooltips use inverted colors on the dark page.
document.documentElement.setAttribute('data-bs-theme', 'dark')
document.documentElement.setAttribute('data-un-theme', 'dark')

export const theme = { resolved: 'dark' as const, tooltip: 'light' as const }
