// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from '@playwright/test'

test('open machine', async ({ page }) => {
  await page.goto('/')

  await page.getByRole('link', { name: 'Clusters' }).click()

  await page.click('#talos-default-cluster-box')
  await page.click('#talos-default-control-planes > div:last-child')
  await page.locator('#etcd').waitFor()

  await page.getByRole('link', { name: 'Clusters' }).click()

  await page.click('#talos-default-workers > div:last-child')
  await page.locator('#machined').waitFor()

  await expect(page.getByText('etcd')).toHaveCount(0)
})
