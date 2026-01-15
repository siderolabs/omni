// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from './cluster_fixtures'

test('View all machines', async ({ cluster, page }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'Machines' }).click()

  await expect(page).toHaveURL('/machines')
  await expect(page.getByRole('heading', { name: 'Machines', exact: true })).toBeVisible()

  await page.getByRole('textbox').fill(cluster.name)

  await expect(page.getByRole('link', { name: cluster.name })).toHaveCount(3)
})
