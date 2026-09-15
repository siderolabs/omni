// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { milliseconds } from 'date-fns'

import { expect, test } from './cluster_fixtures'

// The Security view is only offered when the primary image factory is an enterprise one
// (FeaturesConfigSpec.is_enterprise_image_factory) and the cluster runs Talos >= 1.13.0. Both
// tests below use the newer version, so the community one pins the gate on the factory rather
// than on the Talos version.
test.use({ talosVersion: 'v1.13.10' })

test.describe.configure({ mode: 'parallel' })

test('View cluster vulnerabilities', { tag: '@enterprise-factory' }, async ({ cluster, page }) => {
  test.slow()

  await test.step('Open the cluster Security page', async () => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Clusters' }).click()
    await page.getByRole('link', { name: cluster.name }).click()

    await page.getByRole('link', { name: 'Security' }).click()
  })

  await test.step('Assert the page loaded in enterprise mode', async () => {
    await expect(
      page.getByRole('heading', { name: `Vulnerabilities for ${cluster.name}` }),
    ).toBeVisible()

    // Both of these render instead of the report when the factory, or the Talos version served
    // by it, is not an enterprise one - their absence is what proves the run is enterprise.
    await expect(
      page.getByText('Vulnerability scanning requires the enterprise image factory'),
    ).toBeHidden()
    await expect(
      page.getByText(
        'Vulnerability scanning requires using a talos version from the enterprise image factory',
      ),
    ).toBeHidden()
  })

  const target = page.getByRole('article').first()

  await test.step('Assert a schematic target is scanned', async () => {
    await expect(target.getByRole('heading', { name: /^Schematic/ })).toBeVisible()
    await expect(target.getByText(/applies to \d+ machines?/)).toBeVisible()

    // The scan is a live call out to the factory, so give it room.
    await expect(target.getByText('Running scan…')).toBeHidden({
      timeout: milliseconds({ minutes: 2 }),
    })
  })

  await test.step('Open the scan report', async () => {
    await target.getByRole('button', { name: 'View report' }).click()

    const modal = page.getByRole('dialog', { name: 'Scan details' })
    await expect(modal).toBeVisible()
    await expect(modal.getByRole('button', { name: 'SBOM' })).toBeVisible()
    await expect(modal.getByRole('button', { name: 'VEX' })).toBeVisible()

    await modal.getByRole('button', { name: 'Close', exact: true }).click()
    await expect(modal).toBeHidden()
  })
})

test(
  'Security page is not offered without an enterprise factory',
  { tag: '@community-factory' },
  async ({ cluster, page }) => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Clusters' }).click()
    await page.getByRole('link', { name: cluster.name }).click()

    await expect(page.getByRole('link', { name: 'Overview', exact: true })).toBeVisible()
    await expect(page.getByRole('link', { name: 'Security' })).toBeHidden()
  },
)
