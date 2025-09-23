// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { milliseconds } from 'date-fns'
import { diff as diffJSON } from 'json-diff-ts'
import * as uuid from 'uuid'
import * as yaml from 'yaml'

import { expect, test } from './omnictl_fixtures.js'

test.describe.configure({ mode: 'serial', retries: 0 })

const clusterName = 'talos-test-cluster'
const machineName = 'deadbeef'

test('create cluster', async ({ page }) => {
  test.setTimeout(milliseconds({ minutes: 15 }))

  await page.goto('/')

  await page.getByRole('link', { name: 'Clusters' }).click()
  await page.getByRole('button', { name: 'Create Cluster' }).click()

  // There is some code to put a default value in the input which we must wait for to prevent being overridden
  await expect(page.getByRole('textbox', { name: 'Cluster Name:' })).toHaveValue(/^talos-default/)
  await page.getByRole('textbox', { name: 'Cluster Name' }).fill(clusterName)

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
   hostname: ${machineName}`)

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

  await expect(async () => {
    await expect(page.locator('#machine-count')).toHaveText(/\d\/3/)
    await expect(page.getByText(machineName)).toBeVisible()
    await expect(page.locator('#machine-set-phase-name').getByText('Running')).toHaveCount(2)
    await expect(page.locator('#cluster-machine-stage-name').getByText('Running')).toHaveCount(3)
  }, 'Wait for cluster to be running').toPass({
    intervals: [milliseconds({ seconds: 5 })],
    timeout: milliseconds({ minutes: 15 }),
  })

  // Check that extensions are added
  await page.getByRole('link', { name: machineName }).click()
  await page.getByRole('link', { name: 'Extensions' }).click()

  await expect(page.getByText('siderolabs/usb-modem-drivers')).toBeVisible()
})

test('expand and collapse cluster', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'Clusters' }).click()

  await expect(page.getByText(machineName)).toBeInViewport()

  await page.getByRole('button', { name: clusterName }).click()
  await expect(page.getByText(machineName)).not.toBeInViewport()

  await page.getByRole('button', { name: clusterName }).click()
  await expect(page.getByText(machineName)).toBeInViewport()
})

test('open machine', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'Clusters' }).click()

  const servicesList = page.getByRole('region', { name: 'Services' })

  // Open control plane machine
  await page.getByRole('region', { name: 'Control Planes' }).getByRole('link').last().click()
  await expect(servicesList.getByRole('link', { name: 'etcd' })).toBeVisible()

  await page.getByRole('link', { name: 'Clusters' }).click()

  // Open worker machine
  await page.getByRole('region', { name: 'Workers' }).getByRole('link').last().click()
  await expect(servicesList.getByRole('link', { name: 'machined' })).toBeVisible()
  await expect(servicesList.getByRole('link', { name: 'etcd' })).toBeHidden()
})

test('cluster template export and sync', async ({ omnictl }, testInfo) => {
  test.setTimeout(milliseconds({ minutes: 5 }))

  const templatePath = testInfo.outputPath('cluster.yaml')

  // export a template
  await expect(
    omnictl(['cluster', 'template', 'export', '-c', clusterName, '-o', templatePath, '-f']),
  ).resolves.not.toThrow()

  // collect resources before syncing the template back to the cluster
  const { stdout: clusterBefore } = await omnictl(['get', 'cluster', clusterName, '-ojson'])
  const { stdout: configPatchesBefore } = await omnictl([
    'get',
    'configpatch',
    '-l',
    `omni.sidero.dev/cluster=${clusterName}`,
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
  expect.soft(firstLine).toBe(`* updating Clusters.omni.sidero.dev(${clusterName})`)

  const reg = /\* updating ConfigPatches\.omni\.sidero\.dev\(400-cm-(.*?)\)/
  expect.soft(secondLine).toMatch(reg)

  const machineID = secondLine.match(reg)?.[1]
  expect.soft(() => uuid.parse(machineID ?? ''), 'Expect a valid UUID').not.toThrow()

  // assert the resource manifests are semantically equal before and after export
  const { stdout: clusterAfter } = await omnictl(['get', 'cluster', clusterName, '-ojson'])
  const { stdout: configPatchesAfter } = await omnictl([
    'get',
    'configpatch',
    '-l',
    `omni.sidero.dev/cluster=${clusterName}`,
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

    const ymlObjBefore = yaml.parse(configPatchBefore.spec.data)
    const ymlObjAfter = yaml.parse(configPatchAfter.spec.data)

    expect(ymlObjBefore).toStrictEqual(ymlObjAfter)
  })
})
