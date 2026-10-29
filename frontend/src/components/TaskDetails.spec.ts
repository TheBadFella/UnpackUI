import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import { expect, it } from 'vitest'
import TaskDetails from './TaskDetails.svelte'

it('moves focus into task details and keeps Tab within its controls', async () => {
  render(TaskDetails, {
    item: {
      id: 'fixture', app: 'Folder', path: '/tmp/fixture', outputPath: '',
      url: '', status: 'queued', retries: 0, updated: '', progress: '', error: '',
    },
    onclose: () => {},
  })
  const dialog = await screen.findByRole('dialog')
  await waitFor(() => expect(dialog.contains(document.activeElement)).toBe(true))
  expect(dialog.getAttribute('aria-modal')).toBe('true')
  const buttons = dialog.querySelectorAll('button')
  const first = buttons[0]
  const last = buttons[buttons.length - 1]
  last.focus()
  await fireEvent.keyDown(last, { key: 'Tab' })
  expect(document.activeElement).toBe(first)
  await fireEvent.keyDown(first, { key: 'Tab', shiftKey: true })
  expect(document.activeElement).toBe(last)
})
