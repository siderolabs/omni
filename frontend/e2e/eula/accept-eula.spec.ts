// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from '@playwright/test'

test('accept EULA', async ({ page }) => {
  await page.goto('/')

  await expect(page.getByRole('heading', { name: 'End User License Agreement' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Accept' })).toBeDisabled()

  await page.getByRole('textbox', { name: 'Full Name:' }).fill('Test User')
  await page.getByRole('textbox', { name: 'Email Address:' }).fill('test-user@siderolabs.com')
  await page.getByRole('checkbox', { name: 'I have read and agree to the' }).check()

  await page.getByRole('button', { name: 'Accept' }).click()

  await expect(page.getByText(/Log in to [\w\s]+ to continue to [\w\s]+./)).toBeVisible()
})
