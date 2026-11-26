// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'

import {
  formatSince,
  healthAppearance,
  healthName,
  stateAppearance,
  stateName,
  waitingText,
} from './containerStatus'

describe('stateAppearance', () => {
  test.each([
    ['running', 'healthy', { tone: 'success', glyph: 'success' }],
    ['succeeded', undefined, { tone: 'success', glyph: 'success' }],
    // in progress is info, whatever the health rollup says
    ['pending', 'pending', { tone: 'info', glyph: 'progress' }],
    ['pulling', 'pulling', { tone: 'info', glyph: 'progress' }],
    ['stopping', 'healthy', { tone: 'info', glyph: 'progress' }],
    ['backoff', 'degraded', { tone: 'danger', glyph: 'danger' }],
    // still running, but its health check fails
    ['running', 'degraded', { tone: 'warning', glyph: 'warning' }],
    ['starting', 'degraded', { tone: 'warning', glyph: 'warning' }],
    ['something-new', undefined, { tone: 'info', glyph: 'unknown' }],
  ])('%s/%s', (state, health, expected) => {
    expect(stateAppearance(state, health)).toEqual(expected)
  })
})

test.each([
  ['running', 'Running'],
  ['backoff', 'Backoff'],
  ['succeeded', 'Succeeded'],
  ['something-new', 'something-new'],
  ['', 'Unknown'],
  [undefined, 'Unknown'],
])('stateName(%s)', (state, expected) => {
  expect(stateName(state)).toBe(expected)
})

test.each([
  [undefined, undefined],
  [[], undefined],
  [['path: /run/x'], 'path: /run/x'],
  [['path: /run/x', 'volume: data', 'network'], 'path: /run/x +2'],
])('waitingText(%j)', (waitingFor, expected) => {
  expect(waitingText(waitingFor)).toBe(expected)
})

test.each([
  [41, '41s'],
  [3 * 60 + 12, '3m'],
  [3 * 3600 + 5 * 60, '3h 5m'],
  [2 * 3600, '2h'],
  [12 * 86400 + 6 * 3600 + 59, '12d 6h'],
  [-5, '0s'],
])('formatSince(%i seconds)', (seconds, expected) => {
  const now = new Date('2026-11-19T12:00:00Z')

  expect(formatSince(new Date(now.getTime() - seconds * 1000), now)).toBe(expected)
})

test.each([
  ['healthy', 'Healthy'],
  ['degraded', 'Degraded'],
  ['something-new', 'something-new'],
  [undefined, 'Unknown'],
])('healthName(%s)', (health, expected) => {
  expect(healthName(health)).toBe(expected)
})

test.each([
  ['healthy', { tone: 'success', glyph: 'success' }],
  ['degraded', { tone: 'warning', glyph: 'warning' }],
  ['pending', { tone: 'info', glyph: 'progress' }],
  ['pulling', { tone: 'info', glyph: 'progress' }],
  ['something-new', { tone: 'info', glyph: 'unknown' }],
])('healthAppearance(%s)', (health, expected) => {
  expect(healthAppearance(health)).toEqual(expected)
})
