// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { milliseconds } from 'date-fns'
import { diff as diffJSON } from 'json-diff-ts'
import { parse as parseUUID } from 'uuid'
import { parse as parseYAML } from 'yaml'

import { expect, test } from './omnictl_fixtures.js'

// These tests are slow, serial and non-retryable
test.describe.configure({ mode: 'serial', retries: 0, timeout: milliseconds({ minutes: 15 }) })

test('create cluster', async ({ page }) => {
  await page.goto('/')

  await page.getByRole('link', { name: 'Clusters' }).click()
  await page.getByRole('button', { name: 'Create Cluster' }).click()

  // Add 1 CP and 1 worker
  await page.getByRole('radio', { name: 'CP' }).first().click()
  await page.getByRole('radio', { name: 'W0' }).nth(1).click()

  await test.step('Edit CP config patch', async () => {
    await page.click('button#CP')

    const editor = page.getByRole('textbox', { name: 'Editor content' })

    await editor.press('Control+a')
    await editor.press('Delete')
    await editor.fill(`machine:
 network:
   hostname: deadbeef`)

    await page.getByRole('button', { name: 'Save' }).click()
  })

  await test.step('Set CP extensions', async () => {
    await page.locator('button#extensions-CP').first().click()
    await page.getByText('usb-modem-drivers').click()
    await page.getByRole('button', { name: 'Save' }).click()
  })

  await page.getByRole('button', { name: 'Create Cluster' }).click()

  await test.step('Scale cluster', async () => {
    await page.getByRole('button', { name: 'Cluster Scaling' }).click()
    await page.getByRole('radio', { name: 'W0' }).first().click()
    await page.getByRole('button', { name: 'Update' }).click()

    // Wait for the scaling to navigate successfully to the cluster overview page to avoid a race condition where
    // the test navigates to the Clusters page too early, then the scaling succeeds and goes to the Cluster Overview page
    await page.getByText('Updated Cluster Configuration').waitFor()
  })

  await page.getByRole('link', { name: 'Clusters' }).click()

  // Expand cluster to show machines
  await page
    .locator('div')
    .filter({ hasText: /^talos-default$/ })
    .click()

  await expect(async () => {
    await expect(page.locator('#machine-count')).toHaveText(/\d\/3/)
    await expect(page.getByText('deadbeef')).toBeVisible()
    await expect(page.locator('#machine-set-phase-name').getByText('Running')).toHaveCount(2)
    await expect(page.locator('#cluster-machine-stage-name').getByText('Running')).toHaveCount(3)
  }, 'Wait for cluster to be running').toPass({
    intervals: [milliseconds({ seconds: 5 })],
    timeout: milliseconds({ minutes: 15 }),
  })

  // Check that extensions are added
  await page.getByRole('link', { name: 'deadbeef' }).click()
  await page.getByRole('link', { name: 'Extensions' }).click()

  await expect(page.getByText('siderolabs/usb-modem-drivers')).toBeVisible()
})

// TODO: Make a cluster fixture? As the template test requires the cluster from the previous test to be created
test('cluster template export and sync', async ({ omnictl }, testInfo) => {
  const templatePath = testInfo.outputPath('cluster.yaml')

  // export a template
  await expect(
    omnictl(['cluster', 'template', 'export', '-c', 'talos-default', '-o', templatePath, '-f']),
  ).resolves.not.toThrow()

  // collect resources before syncing the template back to the cluster
  const { stdout: clusterBefore } = await omnictl(['get', 'cluster', 'talos-default', '-ojson'])
  const { stdout: configPatchesBefore } = await omnictl([
    'get',
    'configpatch',
    '-l',
    'omni.sidero.dev/cluster=talos-default',
    '-ojson',
  ])

  // sync the template back to the cluster
  const { stdout: templateSync } = await omnictl([
    'cluster',
    'template',
    'sync',
    '-f',
    templatePath,
  ])

  // assert that only the cluster and a single config patch are updated
  const outputLines = templateSync.trim().split('\n')
  expect.soft(outputLines).toHaveLength(2)

  const [firstLine, secondLine] = outputLines
  expect.soft(firstLine).toBe('* updating Clusters.omni.sidero.dev(talos-default)')

  const reg = /\* updating ConfigPatches\.omni\.sidero\.dev\(400-cm-(.*?)\)/
  expect.soft(secondLine).toMatch(reg)

  const machineID = secondLine.match(reg)?.[1]
  expect.soft(() => parseUUID(machineID ?? ''), 'Expect a valid UUID').not.toThrow()

  // assert the resource manifests are semantically equal before and after export
  const { stdout: clusterAfter } = await omnictl(['get', 'cluster', 'talos-default', '-ojson'])
  const { stdout: configPatchesAfter } = await omnictl([
    'get',
    'configpatch',
    '-l',
    'omni.sidero.dev/cluster=talos-default',
    '-ojson',
  ])

  expect(
    diffJSON(JSON.parse(clusterBefore), JSON.parse(clusterAfter), {
      keysToSkip: ['metadata.updated', 'metadata.version', 'metadata.annotations'],
    }),
  ).toHaveLength(0)

  interface ConfigPatch {
    spec: { data: string }
  }

  const configPatchListBefore = JSON.parse(
    `[${configPatchesBefore.split(/}\s+{/).join('},{')}]`,
  ) as ConfigPatch[]

  const configPatchListAfter = JSON.parse(
    `[${configPatchesAfter.split(/}\s+{/).join('},{')}]`,
  ) as ConfigPatch[]

  expect(configPatchListBefore).toHaveLength(configPatchListAfter.length)

  configPatchListBefore.forEach((configPatchBefore, i) => {
    const configPatchAfter = configPatchListAfter[i]

    expect(
      diffJSON(configPatchBefore, configPatchAfter, {
        keysToSkip: ['metadata.updated', 'metadata.version', 'metadata.annotations', 'spec.data'],
      }),
    ).toHaveLength(0)

    const ymlObjBefore = parseYAML(configPatchBefore.spec.data)
    const ymlObjAfter = parseYAML(configPatchAfter.spec.data)

    expect(ymlObjBefore).toStrictEqual(ymlObjAfter)
  })
})
