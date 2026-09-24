// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from './cluster_fixtures'

test.describe.configure({ mode: 'parallel' })

test('Browse Omni resources', async ({ cluster, page }) => {
  await test.step('Open the Cluster resource type', async () => {
    await page.goto('/internals/resources/omni')

    // Search by the full type, as display names are not unique
    await page.getByRole('textbox', { name: 'Search:' }).fill('Clusters.omni.sidero.dev')
    await page.getByRole('link', { name: 'Cluster', exact: true }).click()

    await expect(page).toHaveURL(/\/internals\/resources\/omni\/Clusters\.omni\.sidero\.dev$/)
  })

  await test.step('Open the cluster resource', async () => {
    await page.getByRole('row', { name: cluster.name }).click()

    await expect(page.getByRole('heading', { name: cluster.name })).toBeVisible()
  })
})

test('Browse Talos resources', async ({ cluster, page }) => {
  await test.step('Pick a machine from the cluster', async () => {
    await page.goto('/internals/resources/talos')

    await page.getByRole('row').filter({ hasText: cluster.name }).first().getByRole('link').click()

    await expect(page.getByRole('link', { name: 'Service', exact: true })).toBeVisible()
  })

  await test.step('Open the Service resource type', async () => {
    // Search by the full type, as display names are not unique
    await page.getByRole('textbox', { name: 'Search:' }).fill('Services.v1alpha1.talos.dev')
    await page.getByRole('link', { name: 'Service', exact: true }).click()

    await expect(page).toHaveURL(
      /\/internals\/resources\/talos\/[^/]+\/Services\.v1alpha1\.talos\.dev$/,
    )
  })

  await test.step('Open the apid service resource', async () => {
    await page.getByRole('row', { name: 'apid' }).click()

    await expect(page.getByRole('heading', { name: 'apid' })).toBeVisible()
  })
})
