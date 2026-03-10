// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { readFile } from 'node:fs/promises'

import { expect, test } from '../auth_fixtures'

test.describe.configure({ mode: 'parallel' })

test('Download audit logs', async ({ page }, testInfo) => {
  await page.goto('/')

  await page.getByRole('link', { name: 'Audit logs' }).click()

  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('button', { name: 'Download audit logs' }).click(),
  ])

  const filePath = testInfo.outputPath(download.suggestedFilename())
  await download.saveAs(filePath)
  await testInfo.attach(download.suggestedFilename(), { path: filePath })

  await expect(readFile(filePath, 'utf-8')).resolves.toContain(
    '"resource_type":"Identities.omni.sidero.dev"',
  )
})

test('Expand audit log item', async ({ page }) => {
  await page.goto('/')

  await page.getByRole('link', { name: 'Audit logs' }).click()

  await expect(
    page.getByRole('row', { name: 'CREATE Identities.omni.sidero.dev System / Service' }),
  ).toBeVisible()

  await test.step('Assert content not visible', async () => {
    await expect(page.getByText('"event_type": "create"')).toBeHidden()
    await expect(page.getByText('"resource_type": "Identities.omni.sidero.dev"')).toBeHidden()
    await expect(page.getByText('"resource_id": "test-user@siderolabs.com"')).toBeHidden()
  })

  await page
    .getByRole('row', { name: 'CREATE Identities.omni.sidero.dev System / Service' })
    .click()

  await test.step('Assert content visible', async () => {
    await expect(page.getByText('"event_type": "create"')).toBeVisible()
    await expect(page.getByText('"resource_type": "Identities.omni.sidero.dev"')).toBeVisible()
    await expect(page.getByText('"resource_id": "test-user@siderolabs.com"')).toBeVisible()
  })

  await page
    .getByRole('row', { name: 'CREATE Identities.omni.sidero.dev System / Service' })
    .click()

  await test.step('Assert content not visible', async () => {
    await expect(page.getByText('"event_type": "create"')).toBeHidden()
    await expect(page.getByText('"resource_type": "Identities.omni.sidero.dev"')).toBeHidden()
    await expect(page.getByText('"resource_id": "test-user@siderolabs.com"')).toBeHidden()
  })
})

test('Filtering audit logs by date', async ({ page }) => {
  await page.goto('/')

  await page.getByRole('link', { name: 'Audit logs' }).click()

  await expect(page.getByRole('row')).not.toHaveCount(0)

  await test.step('Change the date range to a year with no results', async () => {
    await page.getByRole('spinbutton', { name: 'year,' }).first().click()
    await page.keyboard.type('2020')

    await page.getByRole('spinbutton', { name: 'year,' }).nth(1).click()
    await page.keyboard.type('2020')
  })

  await expect(page.getByRole('row')).toHaveCount(0)

  await test.step('Change the date range back to this year', async () => {
    const thisYear = new Date().getFullYear().toString()

    await page.getByRole('spinbutton', { name: 'year,' }).first().click()
    await page.keyboard.type(thisYear)

    await page.getByRole('spinbutton', { name: 'year,' }).nth(1).click()
    await page.keyboard.type(thisYear)
  })

  await expect(page.getByRole('row')).not.toHaveCount(0)
})
