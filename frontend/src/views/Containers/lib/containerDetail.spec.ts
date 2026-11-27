// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from 'vitest'

import {
  dependsOnList,
  healthCheckTarget,
  limitsText,
  mountSource,
  restartText,
  runAsText,
  runSummary,
  runTime,
} from './containerDetail'

test.each([
  [undefined, 'Image default'],
  [{ uid: 1000, gid: 1000 }, '1000:1000'],
  [{ uid: 0 }, '0:image default'],
])('runAsText(%j)', (runAs, expected) => {
  expect(runAsText(runAs)).toBe(expected)
})

test.each([
  [undefined, 'None'],
  [{ cpuLimit: 500, memoryLimit: 256 * 2 ** 20 }, 'CPU 0.5 cores, memory 256 MiB'],
  [{ swapLimit: 0 }, 'no swap'],
  [{ memoryLimit: 2 ** 30, swapLimit: 2 ** 30 }, 'memory 1 GiB, swap 1 GiB'],
])('limitsText(%j)', (resources, expected) => {
  expect(limitsText(resources)).toBe(expected)
})

test.each([
  [undefined, 'Always, no retry limit'],
  [{ policy: 'never' }, 'Never'],
  [
    { policy: 'on-failure', maxRetries: 3, delay: '1s', maxDelay: '1m0s' },
    'On failure, at most 3 retries, backoff 1s to 1m0s',
  ],
])('restartText(%j)', (restart, expected) => {
  expect(restartText(restart)).toBe(expected)
})

test.each([
  [{ kind: 'http', port: 8080, path: '/healthz' }, 'HTTP :8080/healthz'],
  [{ kind: 'http', port: 80 }, 'HTTP :80/'],
  [{ kind: 'tcp', port: 5432 }, 'TCP :5432'],
  [{ kind: 'grpc', port: 9000, grpcService: 'api.Health' }, 'gRPC :9000 api.Health'],
  [{}, 'None'],
])('healthCheckTarget(%j)', (check, expected) => {
  expect(healthCheckTarget(check)).toBe(expected)
})

test.each([
  [{ kind: 'volume', volumeName: 'data' }, 'volume data'],
  [{ kind: 'hostPath', source: '/var/log' }, '/var/log'],
  [{ kind: 'tmpfs' }, 'tmpfs'],
  [{ kind: 'tmpfs', size: 64 * 2 ** 20 }, 'tmpfs, 64 MiB'],
])('mountSource(%j)', (mount, expected) => {
  expect(mountSource(mount)).toBe(expected)
})

test('dependsOnList', () => {
  expect(dependsOnList(undefined)).toEqual([])
  expect(
    dependsOnList({
      networks: ['online'],
      time: true,
      paths: ['/run/x'],
      containers: [{ name: 'db', condition: 'succeeded' }, { name: 'cache' }],
    }),
  ).toEqual([
    'network online',
    'time',
    'path: /run/x',
    'container db succeeded',
    'container cache running',
  ])
})

test.each([
  ['2026-11-26T12:00:00Z', 'Thu, 26 Nov 2026 12:00:00 GMT'],
  ['0001-01-01T00:00:00Z', ''],
  [undefined, ''],
])('runTime(%s)', (time, expected) => {
  expect(runTime(time)).toBe(expected)
})

test('runSummary says every reading the run has', () => {
  expect(
    runSummary({
      pid: 4021,
      exitCode: 1,
      startedAt: '2026-11-26T12:00:00Z',
      finishedAt: '2026-11-26T12:05:00Z',
    }),
  ).toBe(
    'PID 4021, exit code 1, started Thu, 26 Nov 2026 12:00:00 GMT, finished Thu, 26 Nov 2026 12:05:00 GMT',
  )
})

test('runSummary leaves out what a run that never started does not have', () => {
  expect(runSummary({ startedAt: '0001-01-01T00:00:00Z' })).toBe('')
})
