// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from 'vitest'

import type { Resource } from '@/api/grpc'
import type { ContainerStatusSpec } from '@/api/talos/containers.pb'

import { blockedContainers, healthCounts } from './containerSummary'

const status = (id: string, spec: ContainerStatusSpec): Resource<ContainerStatusSpec> => ({
  metadata: { id },
  spec,
})

test('healthCounts counts worst first and leaves out the ones nobody has', () => {
  expect(
    healthCounts([
      status('a', { health: 'healthy' }),
      status('b', { health: 'pending' }),
      status('c', { health: 'healthy' }),
      status('d', { health: 'degraded' }),
      status('e', { health: 'something-new' }),
    ]),
  ).toEqual([
    { health: 'degraded', count: 1 },
    { health: 'pending', count: 1 },
    { health: 'healthy', count: 2 },
    { health: 'something-new', count: 1 },
  ])
})

test('blockedContainers names each waiting container and its first gate', () => {
  expect(
    blockedContainers([
      status('web', { state: 'running' }),
      status('backup-agent', { state: 'pending', waitingFor: ['volume backup-cache'] }),
      status('agent', { state: 'pending', waitingFor: ['time', 'path: /run/x'] }),
    ]),
  ).toEqual([
    { id: 'agent', gate: 'time', more: 1 },
    { id: 'backup-agent', gate: 'volume backup-cache', more: 0 },
  ])
})
