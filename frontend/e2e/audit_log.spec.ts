// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { readFile } from 'fs/promises'

import { expect, test } from './fixtures.js'

test('audit log', async ({ page, loginUser }) => {
  const downloadEvent = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Get audit logs' }).click()

  const download = await downloadEvent
  const filePath = await download.path()
  const fileContent = await readFile(filePath, 'utf-8')

  expect(fileContent).toContain('"resource_type":"Identities.omni.sidero.dev"')
})
