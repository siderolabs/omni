// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from 'vitest'

import { unquoteGo } from './goQuote'

test.each([
  ['"plain"', 'plain'],
  ['"a\\nb\\tc"', 'a\nb\tc'],
  ['"say \\"hi\\" \\\\ bye"', 'say "hi" \\ bye'],
  ['"\\x41\\u00e9\\U0001F600"', 'Aé😀'],
  ['"\\x1b[31mERR\\x1b[0m\\a\\v"', '\x1b[31mERR\x1b[0m\x07\v'],
])('unquoteGo(%s)', (quoted, expected) => {
  expect(unquoteGo(quoted)).toBe(expected)
})
