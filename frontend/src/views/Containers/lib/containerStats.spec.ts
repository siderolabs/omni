// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from 'vitest'

import { containerMetrics, formatCPU, statsKey, statsSample } from './containerStats'

test('statsKey', () => {
  expect(statsKey('web', 3)).toBe('web-3')
  expect(statsKey('web')).toBe('web-0')
})

test('statsSample skips stats without an ID and keeps what is reported', () => {
  const sample = statsSample(
    [
      { id: 'web-1', cpu_usage: '1000', memory_usage: '2048' },
      { id: 'crash-7', memory_usage: '512' },
      { cpu_usage: '5' },
    ],
    1000,
  )

  expect(sample.cpu).toEqual(new Map([['web-1', 1000n]]))
  expect(sample.memory).toEqual(
    new Map([
      ['web-1', 2048],
      ['crash-7', 512],
    ]),
  )
})

test('containerMetrics needs two samples for CPU', () => {
  const first = statsSample([{ id: 'web-1', cpu_usage: '1000000000', memory_usage: '100' }], 0)

  expect(containerMetrics(undefined, first)).toEqual(
    new Map([['web-1', { cpuPercent: undefined, memory: 100 }]]),
  )

  // 0.5s of CPU over 5s is 10% of one core
  const second = statsSample([{ id: 'web-1', cpu_usage: '1500000000', memory_usage: '200' }], 5000)

  expect(containerMetrics(first, second).get('web-1')).toEqual({ cpuPercent: 10, memory: 200 })
})

test('containerMetrics shows no CPU for a new instance or a counter that went back', () => {
  const first = statsSample([{ id: 'web-1', cpu_usage: '900' }], 0)
  const second = statsSample(
    [
      { id: 'web-1', cpu_usage: '100' },
      { id: 'web-2', cpu_usage: '100' },
    ],
    5000,
  )

  const metrics = containerMetrics(first, second)

  expect(metrics.get('web-1')?.cpuPercent).toBeUndefined()
  expect(metrics.get('web-2')?.cpuPercent).toBeUndefined()
})

test.each([
  [0, '0.0%'],
  [1.234, '1.2%'],
  [9.96, '10%'],
  [9.94, '9.9%'],
  [42.6, '43%'],
  [180, '180%'],
])('formatCPU(%d)', (percent, expected) => {
  expect(formatCPU(percent)).toBe(expected)
})
