// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import fs from 'node:fs/promises'
import os from 'node:os'

import type { Page } from '@playwright/test'
import { milliseconds } from 'date-fns'
import { diff as diffJSON } from 'json-diff-ts'
import * as uuid from 'uuid'
import * as yaml from 'yaml'

import { expect, test } from '../omnictl_fixtures.js'

test.describe.configure({ mode: 'serial', retries: 0 })

const clusterName = 'talos-test-cluster'
const cpMachineName = 'deadbeef'

test('create cluster', async ({ page }) => {
  test.setTimeout(milliseconds({ minutes: 16 }))

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
    await editor.pressSequentially(`---
apiVersion: v1alpha1
kind: HostnameConfig
auto: off
hostname: ${cpMachineName}`)

    await page.getByRole('button', { name: 'Save' }).click()
  })

  await test.step('Set CP extensions', async () => {
    await page.locator('button#extensions-CP').first().click()
    await page.getByText('usb-modem-drivers').click()
    await page.getByRole('button', { name: 'Save' }).click()
  })

  await page.getByRole('button', { name: 'Create Cluster' }).click()

  await test.step('Scale cluster', async () => {
    await page.getByRole('link', { name: 'Cluster Scaling' }).click()
    await page.getByRole('radio', { name: 'W0' }).first().click()
    await page.getByRole('button', { name: 'Update' }).click()

    // Wait for the scaling to navigate successfully to the cluster overview page to avoid a race condition where
    // the test navigates to the Clusters page too early, then the scaling succeeds and goes to the Cluster Overview page
    await page.getByText('Updated Cluster Configuration').waitFor()
  })

  await page.getByRole('link', { name: 'Clusters' }).click()

  await expect(async () => {
    await expect(
      page.getByRole('button', { name: clusterName }).getByTestId('machine-count'),
    ).toHaveText(/\d\/3/)
    await expect(page.getByText(cpMachineName)).toBeVisible()
    await expect(page.getByTestId('machine-set-phase-name').getByText('Running')).toHaveCount(2)
    await expect(page.getByTestId('cluster-machine-stage-name').getByText('Running')).toHaveCount(3)
  }, 'Wait for cluster to be running').toPass({
    intervals: [milliseconds({ seconds: 5 })],
    timeout: milliseconds({ minutes: 15 }),
  })

  // Check that extensions are added
  await page.getByRole('link', { name: cpMachineName }).click()
  await page.getByRole('tab', { name: 'Extensions' }).click()

  await expect(page.getByText('siderolabs/usb-modem-drivers')).toBeVisible()
})

test('expand and collapse cluster', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'Clusters' }).click()

  await expect(page.getByText(cpMachineName)).toBeInViewport()

  await page.getByRole('button', { name: clusterName }).click()
  await expect(page.getByText(cpMachineName)).not.toBeInViewport()

  await page.getByRole('button', { name: clusterName }).click()
  await expect(page.getByText(cpMachineName)).toBeInViewport()
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

test('exposed services', async ({ page }, testInfo) => {
  await test.step('Visit cluster overview', async () => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Clusters' }).click()

    await expect(page).toHaveURL('/clusters')
    await expect(page.getByRole('heading', { name: 'Clusters' })).toBeVisible()

    await page.getByRole('link', { name: clusterName, exact: true }).click()
    await expect(page.getByRole('heading', { name: clusterName, exact: true })).toBeVisible()
  })

  await test.step('Enable workload service proxying', async () => {
    await page.getByText('Workload Service Proxying').click()
    await expect(page.getByRole('checkbox', { name: 'Workload Service Proxying' })).toBeChecked()
  })

  await test.step('Visit config patches for control plane', async () => {
    await page.getByRole('link', { name: cpMachineName }).click()
    await page.getByRole('tab', { name: 'Patches', exact: true }).click()
    await page.getByRole('button', { name: 'Create Patch' }).click()
  })

  await test.step('Add service via inlineManifests patch', async () => {
    const cpPatch = await fs.readFile(new URL('./e2e_nginx.yaml', import.meta.url), 'utf8')
    await testInfo.attach('inline_manifest_patch.yaml', {
      body: cpPatch,
      contentType: 'application/yaml',
    })

    await page.evaluate((text) => navigator.clipboard.writeText(text), cpPatch)
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(cpPatch)

    await page
      .getByRole('textbox', { name: 'Editor content' })
      .press(`${os.platform() === 'darwin' ? 'Meta' : 'Control'}+v`)
    await expect(page.getByText('inlineManifests:')).toBeVisible()

    await page.getByRole('button', { name: 'Save' }).click()
  })

  await test.step('Visit cluster overview', async () => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Clusters' }).click()

    await expect(page).toHaveURL('/clusters')
    await expect(page.getByRole('heading', { name: 'Clusters' })).toBeVisible()

    await page.getByRole('link', { name: clusterName, exact: true }).click()
    await expect(page.getByRole('heading', { name: clusterName, exact: true })).toBeVisible()
  })

  await expect(async () => {
    let servicePage: Page | undefined

    try {
      ;[servicePage] = await Promise.all([
        page.waitForEvent('popup'),
        page
          .getByRole('link', { name: 'E2E Nginx' })
          .click({ timeout: milliseconds({ minutes: 1 }) }),
      ])

      await expect(servicePage.getByRole('heading', { name: 'Welcome to nginx!' })).toBeVisible()
    } finally {
      await servicePage?.close()
    }
  }, 'Wait for service to be running').toPass({
    intervals: [milliseconds({ seconds: 5 })],
    timeout: milliseconds({ minutes: 1 }),
  })
})

test('node overview tabs', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'Clusters' }).click()

  const servicesList = page.getByRole('region', { name: 'Services' })

  // Open control plane machine
  await page.getByRole('region', { name: 'Control Planes' }).getByRole('link').last().click()
  await expect(servicesList.getByRole('link', { name: 'etcd' })).toBeVisible()

  await test.step('Validate monitor tab', async () => {
    await page.getByRole('tab', { name: 'Monitor', exact: true }).click()
    await expect(page.getByText('CPU usage')).toBeVisible()
    await expect(page.getByText('Memory', { exact: true })).toBeVisible()
    await expect(page.getByText('Processes')).toBeVisible()
    await expect(page.getByText('init /sbin/init')).toBeVisible()
  })

  await test.step('Validate console logs tab', async () => {
    await page.getByRole('tab', { name: 'Console Logs', exact: true }).click()
    await expect(page.getByText('[talos]').first()).toBeVisible()
  })

  await test.step('Validate config tab', async () => {
    await page.getByRole('tab', { name: 'Config', exact: true }).click()
    await expect(page.getByText('version: v1alpha1').first()).toBeVisible()
  })

  await test.step('Validate pending updates tab', async () => {
    await page.getByRole('tab', { name: 'Pending Updates', exact: true }).click()
    await expect(page.getByText('No pending config updates found for this machine')).toBeVisible()
  })

  await test.step('Validate config history tab', async () => {
    await page.getByRole('tab', { name: 'Config History', exact: true }).click()
    await expect(page.getByText('inlineManifests:')).toBeVisible()

    // This asserts that the virtualisation is working
    await expect(page.getByText('targetPort: 80')).toBeHidden()
    await page.getByRole('textbox', { name: 'Search:' }).fill('targetPort:')
    await expect(page.getByText('targetPort: 80')).toBeVisible()
  })

  await test.step('Validate patches tab', async () => {
    await page.getByRole('tab', { name: 'Patches', exact: true }).click()
    await expect(page.getByText('This cluster is managed using cluster templates.')).toBeVisible()
    await expect(page.getByText(`Cluster Machine: ${cpMachineName}`)).toBeVisible()
    await expect(page.getByText(/400-cm-\w+/).first()).toBeVisible()
    await expect(page.getByText('User defined patch')).toBeVisible()
  })

  await test.step('Validate disks tab', async () => {
    await page.getByRole('tab', { name: 'Disks', exact: true }).click()

    const card = page
      .getByRole('region', { name: 'vda' })
      .getByRole('region', { name: 'EPHEMERAL' })

    await expect(card).toBeVisible()
    await expect(card.getByText('Filesystem:xfs')).toBeVisible()
    await expect(card.getByText('Encryption:disabled')).toBeVisible()
  })

  await test.step('Validate extensions tab', async () => {
    await page.getByRole('tab', { name: 'Extensions', exact: true }).click()
    await expect(page.getByText('siderolabs/hello-world-service')).toBeVisible()
    await expect(page.getByText('siderolabs/usb-modem-drivers')).toBeVisible()
  })

  await page.getByRole('link', { name: 'Clusters' }).click()

  // Open worker machine
  await page.getByRole('region', { name: 'Workers' }).getByRole('link').last().click()
  await expect(servicesList.getByRole('link', { name: 'machined' })).toBeVisible()
  await expect(servicesList.getByRole('link', { name: 'etcd' })).toBeHidden()
})

test('destroy cluster', async ({ page }) => {
  test.setTimeout(milliseconds({ minutes: 3 }))

  await test.step('Visit clusters page', async () => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Clusters' }).click()
  })

  const row = page.getByRole('button', { name: clusterName })

  await test.step('Destroy cluster', async () => {
    await row.getByRole('button', { name: 'cluster actions' }).click()

    await page.getByRole('menuitem', { name: 'Destroy Cluster' }).click()
    await page.getByRole('button', { name: 'Destroy', exact: true }).click()
  })

  await test.step('Wait for cluster to be destroyed', async () => {
    await expect(page.getByText(`The Cluster ${clusterName} is tearing down`)).toBeVisible()
    await expect(row.getByText('Destroying')).toBeVisible()
    await expect(row).toBeHidden({ timeout: milliseconds({ minutes: 2 }) })
  })
})
