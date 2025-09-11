// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from '@playwright/test'

test('click view all', async ({ page }) => {
  await page.goto('/')

  await page
    .locator('section')
    .filter({ hasText: 'Recent Clusters' })
    .getByRole('button', { name: 'View All' })
    .click()

  await expect(page).toHaveURL('/clusters')
})
