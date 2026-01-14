// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from './cluster_fixtures'

test('View all clusters', async ({ cluster, page }) => {
  await page.goto('/')

  await page
    .locator('section')
    .filter({ hasText: 'Recent Clusters' })
    .getByRole('button', { name: 'View All' })
    .click()

  await expect(page).toHaveURL('/clusters')
  await expect(page.getByRole('heading', { name: 'Clusters' })).toBeVisible()

  await expect(page.getByText(cluster.name, { exact: true })).toBeVisible()
})
