import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import { expect, it, vi } from 'vitest'

const config = vi.hoisted(() => ({
  loadSection: vi.fn(async () => ({ data: {
    parallel: 1, logFileMb: 10, progress: '15s', passwords: [],
    fileMode: '0644', dirMode: '0755', logFileMode: '0600', timeout: '10s',
  } })),
  saveSection: vi.fn(async (..._args: unknown[]) => true),
}))
vi.mock('../../lib/config', () => config)
vi.mock('../../lib/auth.svelte', () => ({
  has: () => true,
  profile: { info: { goos: 'linux' } },
}))
import GeneralForm from './GeneralForm.svelte'

it('saves log permissions and the default request timeout', async () => {
  render(GeneralForm)
  await fireEvent.change(await screen.findByLabelText('Log file permissions'), {
    target: { value: '0640' },
  })
  await fireEvent.change(screen.getByLabelText('Default request timeout'), {
    target: { value: '30s' },
  })
  await fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(config.saveSection).toHaveBeenCalledWith(
    'general', expect.objectContaining({ logFileMode: '0640', timeout: '30s' }),
  ))
})
