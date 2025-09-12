// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from './fixtures.js'

test('title', async ({ page, loginUser }) => {
  await expect(page).toHaveTitle('Omni - default')
})
