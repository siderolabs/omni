// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from 'vitest'

import { stripAnsiColours } from './ansi'

test.each([
  ['plain', 'plain'],
  ['\x1b[31merror\x1b[0m open', 'error open'],
  ['\x1b[1;32mOK\x1b[m', 'OK'],
])('stripAnsiColours(%j)', (text, expected) => {
  expect(stripAnsiColours(text)).toBe(expected)
})
