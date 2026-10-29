import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import { beforeEach, expect, it, vi } from 'vitest'
import { env } from '../../lib/env.svelte'

const config = vi.hoisted(() => ({
  loadSection: vi.fn(),
  loadSectionLive: vi.fn(),
  saveSection: vi.fn(async () => true),
}))
vi.mock('../../lib/config', () => config)
vi.mock('../../lib/auth.svelte', () => ({ has: () => true }))
import FoldersForm from './FoldersForm.svelte'

beforeEach(() => {
  vi.clearAllMocks()
  env.pairs = {}
  config.loadSection.mockResolvedValue({ data: { buffer: 20000, folder: {} } })
  config.loadSectionLive.mockResolvedValue({ data: { folder: {} } })
})

it('saves the extraction-log preference for a watched folder', async () => {
  render(FoldersForm)
  await fireEvent.click(await screen.findByRole('button', { name: 'Add folder' }))
  const path = screen.getByLabelText('Watch path')
  await fireEvent.input(path, { target: { value: '/tmp/archives' } })
  const log = screen.getByLabelText('Create extraction log')
  await fireEvent.change(log, { target: { value: 'true' } })
  await fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(config.saveSection).toHaveBeenCalled())
  const payload = config.saveSection.mock.calls[0] as unknown as [string, { folder: Record<string, { disable_log: boolean }> }]
  expect(payload[0]).toBe('folders')
  expect(Object.values(payload[1].folder)[0].disable_log).toBe(true)
})

it('locks an environment-owned extraction-log preference', async () => {
  env.pairs = { FOLDER_archives_DISABLE_LOG: 'true' }
  config.loadSection.mockResolvedValue({ data: { buffer: 20000, folder: { archives: { path: '/tmp/archives', disable_log: false } } } })
  render(FoldersForm)
  expect((await screen.findByLabelText('Create extraction log') as HTMLSelectElement).disabled).toBe(true)
})
