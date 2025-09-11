// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { readFile } from 'node:fs/promises'

import { expect, test } from '@playwright/test'

test('audit log', async ({ page }, testInfo) => {
  await page.goto('/')

  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('button', { name: 'Get audit logs' }).click(),
  ])

  const filePath = await download.path()
  await testInfo.attach(download.suggestedFilename(), { path: filePath })

  await expect(readFile(filePath, 'utf-8')).resolves.toContain(
    '"resource_type":"Identities.omni.sidero.dev"',
  )
})
