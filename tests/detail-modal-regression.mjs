import test from 'node:test'
import assert from 'node:assert/strict'
import {
  statusHasExtractResult,
  shouldRefetchDetail,
  isDetailResponseValid,
  mergeQueueDetail,
  visibleFiles,
} from '../frontend/src/lib/dashboard.ts'

test('statusHasExtractResult correctly identifies completion result statuses', () => {
  const resultStatuses = [
    'extracted',
    'extractednothing',
    'extractfailed',
    'imported',
    'deleting',
    'deleted',
    'deletefailed',
    'EXTRACTED',
    'Imported',
  ]
  for (const st of resultStatuses) {
    assert.equal(
      statusHasExtractResult(st),
      true,
      `expected ${st} to have extract result`,
    )
  }

  const nonResultStatuses = [
    'starting',
    'extracting',
    'queued',
    'pending',
    'unknown',
    '',
  ]
  for (const st of nonResultStatuses) {
    assert.equal(
      statusHasExtractResult(st),
      false,
      `expected ${st} NOT to have extract result`,
    )
  }
})

test('shouldRefetchDetail triggers one-shot refetch on transition to result status', () => {
  const openItem = {
    id: 'item-1',
    app: 'Sonarr',
    url: '',
    path: '/downloads/test',
    outputPath: '',
    status: 'extracting',
    retries: 0,
    updated: '2026-10-13T00:00:00Z',
    progress: '50%',
    error: '',
  }

  // Still extracting: do not refetch
  assert.equal(shouldRefetchDetail(openItem, 'extracting', false), false)

  // Transitioned to extracted and not yet ready: refetch
  assert.equal(shouldRefetchDetail(openItem, 'extracted', false), true)

  // Transitioned to extractfailed and not yet ready: refetch
  assert.equal(shouldRefetchDetail(openItem, 'extractfailed', false), true)

  // Already refetched (detailFilesReady is true): do not refetch again
  assert.equal(shouldRefetchDetail(openItem, 'extracted', true), false)

  // Modal closed (open is null): do not refetch
  assert.equal(shouldRefetchDetail(null, 'extracted', false), false)
})

test('isDetailResponseValid prevents stale request races and cross-item leaks', () => {
  // Scenario 1: Active request matches active selection
  assert.equal(isDetailResponseValid(1, 1, 'item-1', 'item-1'), true)

  // Scenario 2: Slow response from request 1 arrives after request 2 was issued
  assert.equal(isDetailResponseValid(1, 2, 'item-1', 'item-1'), false)

  // Scenario 3: Response for item-1 arrives after selection switched to item-2
  assert.equal(isDetailResponseValid(1, 1, 'item-1', 'item-2'), false)

  // Scenario 4: Response arrives after modal was closed (activeItemId undefined)
  assert.equal(isDetailResponseValid(1, 1, 'item-1', undefined), false)
})

test('mergeQueueDetail supplements sparse live queue row with full file lists', () => {
  const pending = {
    id: 'item-1',
    app: 'Radarr',
    path: '/downloads/movie',
    status: 'extracting',
    retries: 0,
    updated: '2026-10-13T00:00:00Z',
    progress: '50%',
    error: '',
    url: '',
    outputPath: '',
  }

  const liveRow = {
    ...pending,
    progress: '75%',
    percent: 75,
  }

  const detailFiles = {
    ...pending,
    newFiles: ['/downloads/movie/movie.mkv'],
    origFiles: ['/downloads/movie/movie.part1.rar', '/downloads/movie/movie.part2.rar'],
    archiveFiles: ['/downloads/movie/movie.part1.rar'],
    extraFiles: ['/downloads/movie/movie.nfo'],
    outputPath: '/downloads/extracted',
    ids: { title: 'Test Movie', tmdbId: 12345 },
  }

  const merged = mergeQueueDetail(pending, liveRow, detailFiles)
  assert.ok(merged)
  assert.equal(merged.id, 'item-1')
  assert.equal(merged.progress, '75%')
  assert.deepEqual(merged.newFiles, ['/downloads/movie/movie.mkv'])
  assert.deepEqual(merged.origFiles, [
    '/downloads/movie/movie.part1.rar',
    '/downloads/movie/movie.part2.rar',
  ])
  assert.deepEqual(merged.extraFiles, ['/downloads/movie/movie.nfo'])
  assert.equal(merged.outputPath, '/downloads/extracted')
  assert.deepEqual(merged.ids, { title: 'Test Movie', tmdbId: 12345 })
})

test('mergeQueueDetail prioritizes completed history file lists and output paths over stale API responses', () => {
  const pending = {
    id: 'item-1',
    app: 'Sonarr',
    path: '/downloads/show',
    status: 'extracting',
    retries: 0,
    updated: '2026-10-13T00:00:00Z',
    progress: '20%',
    error: '',
    url: '',
    outputPath: '/downloads/old-output',
  }

  // Stale preliminary API response fetched early while extracting
  const staleDetailFiles = {
    ...pending,
    newFiles: [],
    origFiles: ['/downloads/show/episode.rar'],
    outputPath: '/downloads/old-output',
  }

  // Authoritative completed record from history
  const historyRow = {
    ...pending,
    status: 'imported',
    finished: '2026-10-13T00:05:00Z',
    newFiles: ['/downloads/final/episode.mkv'],
    origFiles: ['/downloads/show/episode.rar'],
    outputPath: '/downloads/final',
  }

  const merged = mergeQueueDetail(pending, historyRow, staleDetailFiles)
  assert.ok(merged)
  assert.equal(merged.status, 'imported')
  assert.equal(merged.outputPath, '/downloads/final')
  assert.deepEqual(merged.newFiles, ['/downloads/final/episode.mkv'])
})

test('mergeQueueDetail treats explicit empty final result as authoritative (no resurrection of stale files)', () => {
  const pending = {
    id: 'item-nothing',
    app: 'Folder',
    path: '/downloads/watched/empty-folder',
    status: 'extracting',
    retries: 0,
    updated: '2026-10-13T00:00:00Z',
    progress: '50%',
    error: '',
    url: '',
    outputPath: '/downloads/old-output',
  }

  // Stale preliminary API response fetched earlier with files and old output path
  const staleDetailFiles = {
    ...pending,
    newFiles: ['/downloads/old-output/stale-extracted.mkv'],
    origFiles: ['/downloads/watched/empty-folder/old.rar'],
    outputPath: '/downloads/old-output',
  }

  // Authoritative completed record with status 'extractednothing' and explicitly empty file lists and empty outputPath
  const completedRow = {
    ...pending,
    status: 'extractednothing',
    finished: '2026-10-13T00:02:00Z',
    newFiles: [],
    origFiles: [],
    outputPath: '',
  }

  const merged = mergeQueueDetail(pending, completedRow, staleDetailFiles)
  assert.ok(merged)
  assert.equal(merged.status, 'extractednothing')
  // Must be empty array, NOT resurrected staleDetailFiles.newFiles
  assert.deepEqual(merged.newFiles, [])
  // Must be empty array, NOT resurrected staleDetailFiles.origFiles
  assert.deepEqual(merged.origFiles, [])
  // Must be empty string, NOT resurrected staleDetailFiles.outputPath
  assert.equal(merged.outputPath, '')
})

test('visibleFiles excludes marker files from display and downloads', () => {
  const files = [
    '/downloads/movie/movie.mkv',
    '/downloads/movie/_unpackerred.txt',
    '/downloads/movie/unpackerred.log',
    '/downloads/movie/_unpackerred',
    '/downloads/movie/sample.avi',
  ]

  const filtered = visibleFiles(files)
  assert.deepEqual(filtered, [
    '/downloads/movie/movie.mkv',
    '/downloads/movie/sample.avi',
  ])
})
