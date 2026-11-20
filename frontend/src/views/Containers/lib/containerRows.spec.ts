// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'

import type { ContainerStatusSpec } from '@/api/talos/containers.pb'

import { type ContainerRow, containerRows, imageRef, sinceTime } from './containerRows'

describe('containerRows', () => {
  const rows = containerRows(
    [
      {
        metadata: { id: 'web' },
        spec: { state: 'running', image: 'sha256:aaa', generation: 3 },
      },
      { metadata: { id: 'waiting' }, spec: { state: 'pending' } },
    ],
    [{ metadata: { id: 'web' }, spec: { image: 'docker.io/library/nginx:1.27' } }],
    [
      { metadata: { id: 'web-1' }, spec: { containerID: 'web', generation: 1, startedAt: 'a' } },
      { metadata: { id: 'web-3' }, spec: { containerID: 'web', generation: 3, startedAt: 'c' } },
      { metadata: { id: 'web-2' }, spec: { containerID: 'web', generation: 2, startedAt: 'b' } },
    ],
    // stats of an earlier instance don't count
    new Map([
      ['web-3', { cpuPercent: 4, memory: 1024 }],
      ['web-2', { cpuPercent: 99 }],
    ]),
  )

  test('joins the image status by ID and keeps the newest instance', () => {
    expect(rows[0].image?.image).toBe('docker.io/library/nginx:1.27')
    expect(rows[0].instance?.generation).toBe(3)
  })

  test('takes the metrics of the current instance', () => {
    expect(rows[0].metrics).toEqual({ cpuPercent: 4, memory: 1024 })
  })

  test('leaves a container without an image status or instance bare', () => {
    expect(rows[1]).toEqual({ id: 'waiting', status: { state: 'pending' } })
    expect(rows[1].metrics).toBeUndefined()
  })

  test('imageRef falls back to the digest', () => {
    expect(imageRef(rows[0])).toBe('docker.io/library/nginx:1.27')
    expect(imageRef({ id: 'x', status: { image: 'sha256:bbb' } })).toBe('sha256:bbb')
  })
})

const row = (
  status: ContainerStatusSpec,
  startedAt?: string,
  finishedAt?: string,
): ContainerRow => ({
  id: 'x',
  status,
  instance: { startedAt, finishedAt },
})

test.each<[ContainerRow, string | undefined]>([
  [
    row({ state: 'running' }, '2026-11-19T10:00:00Z', '0001-01-01T00:00:00Z'),
    '2026-11-19T10:00:00.000Z',
  ],
  [
    row({ state: 'backoff' }, '2026-11-19T10:00:00Z', '2026-11-19T11:00:00Z'),
    '2026-11-19T11:00:00.000Z',
  ],
  [
    row({ state: 'succeeded' }, '2026-11-19T10:00:00Z', '2026-11-19T11:00:00Z'),
    '2026-11-19T11:00:00.000Z',
  ],
  [row({ state: 'exited' }, '2026-11-19T10:00:00Z', '0001-01-01T00:00:00Z'), undefined],
  [row({ state: 'pending' }), undefined],
  [{ id: 'x', status: { state: 'running' } }, undefined],
])('sinceTime(%j)', (r, expected) => {
  expect(sinceTime(r)?.toISOString()).toBe(expected)
})
