// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from 'vitest'

import type { ContainerStatusSpec } from '@/api/talos/containers.pb'

import type { ContainerRow } from './containerRows'
import { type ContainerSort, filterContainers } from './filterContainers'

const row = (
  id: string,
  status: ContainerStatusSpec,
  image?: string,
  metrics?: ContainerRow['metrics'],
): ContainerRow => ({
  id,
  status,
  image: image ? { image } : undefined,
  metrics,
})

const rows = [
  row(
    'web',
    { health: 'healthy', state: 'running', restartCount: 3 },
    'docker.io/library/nginx:1.27',
    { cpuPercent: 5, memory: 100 },
  ),
  row('crashloop', {
    health: 'degraded',
    state: 'backoff',
    restartCount: 12,
    error: 'exit status 1',
  }),
  row('tailscale', { health: 'healthy', state: 'running', restartCount: 0 }, undefined, {
    cpuPercent: 20,
    memory: 50,
  }),
  row('waiting', { health: 'pending', state: 'pending' }),
]

const ids = (list: ContainerRow[]) => list.map((r) => r.id)

test.each<[ContainerSort, string[]]>([
  ['name', ['crashloop', 'tailscale', 'waiting', 'web']],
  ['-name', ['web', 'waiting', 'tailscale', 'crashloop']],
  ['-restarts', ['crashloop', 'web', 'waiting', 'tailscale']],
  ['restarts', ['tailscale', 'waiting', 'web', 'crashloop']],
  // no reading sorts last, by name
  ['-cpu', ['tailscale', 'web', 'crashloop', 'waiting']],
  ['-memory', ['web', 'tailscale', 'crashloop', 'waiting']],
])('sorts by %s', (sort, expected) => {
  expect(ids(filterContainers(rows, { sort }))).toEqual(expected)
})

test.each([
  ['WEB', ['web']],
  ['nginx', ['web']],
  ['backoff', ['crashloop']],
  ['exit status', ['crashloop']],
  ['  ', ['crashloop', 'tailscale', 'waiting', 'web']],
])('searches for %j', (search, expected) => {
  expect(ids(filterContainers(rows, { search }))).toEqual(expected)
})

test('filters by health', () => {
  expect(ids(filterContainers(rows, { health: 'healthy' }))).toEqual(['tailscale', 'web'])
})

test('combines search and health', () => {
  expect(ids(filterContainers(rows, { search: 'tail', health: 'healthy' }))).toEqual(['tailscale'])
})
