// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import fs from 'node:fs/promises'
import os from 'node:os'

import type { Page } from '@playwright/test'
import { milliseconds } from 'date-fns'
import { dump, load } from 'js-yaml'
import { diff as diffJSON } from 'json-diff-ts'
import * as uuid from 'uuid'
import * as yaml from 'yaml'

import { expect, test as base } from '../omnictl_fixtures.js'

const clusterName = 'talos-test-cluster'

const test = base.extend<{ saveSupportBundleOnFailure: void }>({
  saveSupportBundleOnFailure: [
    async ({ omnictl }, use, testInfo) => {
      await use()

      // Save support bundle if the test failed
      if (testInfo.status !== 'passed') {
        const bundlePath = testInfo.outputPath(`support-bundle-${clusterName}.zip`)

        try {
          await omnictl(['support', '--cluster', clusterName, '--output', bundlePath])
          await testInfo.attach(`support-bundle-${clusterName}.zip`, { path: bundlePath })
        } catch (e) {
          console.error(`failed to save support bundle for cluster ${clusterName}:`, e)
        }
      }
    },
    { auto: true },
  ],
})

test.describe.configure({ mode: 'serial', retries: 0 })

const cpMachineName = 'deadbeef'

test('machine overview tabs', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'Machines' }).click()

  // Open first visible machine
  await page
    .getByRole('link', { name: /machine-[-a-f0-9]+/ })
    .first()
    .click()

  await test.step('Validate logs tab', async () => {
    await page.getByRole('tab', { name: 'Logs', exact: true }).click()

    await page.getByPlaceholder('Search').fill('[talos] [initramfs] booting Talos')
    await expect
      .soft(page.getByText(/\[talos\] \[initramfs\] booting Talos v\d\.\d+\.\d+/).first())
      .toBeVisible()

    await page.getByPlaceholder('Search').clear()
    await expect
      .soft(page.getByText(/\[talos\] \[initramfs\] booting Talos v\d\.\d+\.\d+/))
      .toBeHidden()
  })

  await test.step('Validate patches tab', async () => {
    await page.getByRole('tab', { name: 'Patches', exact: true }).click()
    await expect.soft(page.getByRole('heading', { name: 'No Config Patches' })).toBeVisible()
  })

  await test.step('Validate disks tab', async () => {
    await page.getByRole('tab', { name: 'Disks', exact: true }).click()

    await expect
      .soft(page.getByRole('region', { name: 'vda' }).getByText('Unallocated6.00 GB'))
      .toBeVisible()
  })

  await test.step('Validate devices tab', async () => {
    await page.getByRole('tab', { name: 'Devices', exact: true }).click()
    await expect.soft(page.getByText('Ethernet controller')).toBeVisible()
  })

  await test.step('Validate extensions tab', async () => {
    await page.getByRole('tab', { name: 'Extensions', exact: true }).click()
    await expect.soft(page.getByText('siderolabs/hello-world-service')).toBeVisible()
  })
})

test('create cluster', async ({ page }) => {
  test.setTimeout(milliseconds({ minutes: 16 }))

  await page.goto('/')

  await page.getByRole('link', { name: 'Clusters' }).click()
  await page.getByRole('link', { name: 'Create Cluster' }).click()

  // There is some code to put a default value in the input which we must wait for to prevent being overridden
  await expect(page.getByRole('textbox', { name: 'Cluster Name:' })).toHaveValue(/^talos-default/)
  await page.getByRole('textbox', { name: 'Cluster Name' }).fill(clusterName)

  // Add 1 CP and 1 worker
  await page.getByRole('radio', { name: 'CP' }).first().click()
  await page.getByRole('radio', { name: 'W0' }).nth(1).click()

  await test.step('Edit CP config patch', async () => {
    await page.locator('button#CP').click()

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

test('add control plane patch', async ({ page }, testInfo) => {
  await test.step('Visit cluster overview', async () => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Clusters' }).click()

    await expect(page).toHaveURL('/clusters')
    await expect(page.getByRole('heading', { name: 'Clusters', exact: true })).toBeVisible()

    await page.getByRole('link', { name: clusterName, exact: true }).click()
    await expect(page.getByRole('heading', { name: clusterName, exact: true })).toBeVisible()
  })

  await test.step('Visit config patches for control plane', async () => {
    await page.getByRole('link', { name: cpMachineName }).click()
    await page.getByRole('tab', { name: 'Patches', exact: true }).click()
    await page.getByRole('link', { name: 'Create Patch' }).click()
  })

  await test.step('Add EnvironmentConfig patch', async () => {
    const envPatch = await fs.readFile(new URL('./env_config_patch.yaml', import.meta.url), 'utf8')
    await testInfo.attach('env_config_patch.yaml', {
      body: envPatch,
      contentType: 'application/yaml',
    })

    await page.evaluate((text) => navigator.clipboard.writeText(text), envPatch)
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(envPatch)

    await page
      .getByRole('textbox', { name: 'Editor content' })
      .press(`${os.platform() === 'darwin' ? 'Meta' : 'Control'}+v`)
    await expect(page.getByText('variables:')).toBeVisible()

    await page.getByRole('button', { name: 'Save' }).click()
  })
})

test('exposed services', async ({ page, omnictl }, testInfo) => {
  test.setTimeout(milliseconds({ minutes: 2 }))

  await test.step('Visit cluster overview', async () => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Clusters' }).click()

    await expect(page).toHaveURL('/clusters')
    await expect(page.getByRole('heading', { name: 'Clusters', exact: true })).toBeVisible()

    await page.getByRole('link', { name: clusterName, exact: true }).click()
    await expect(page.getByRole('heading', { name: clusterName, exact: true })).toBeVisible()
  })

  await test.step('Enable workload service proxying', async () => {
    await page.getByText('Workload Service Proxying').click()
    await expect(page.getByRole('checkbox', { name: 'Workload Service Proxying' })).toBeChecked()
  })

  await test.step('Visit manifests status', async () => {
    await page.getByRole('link', { name: 'Manifests Status' }).click()

    await expect(
      page.getByRole('heading', { name: `Manifests Status — ${clusterName}` }),
    ).toBeVisible()
  })

  await test.step('Verify workload proxy manifest applied', async () => {
    const workloadProxyRow = page.getByRole('row', {
      name: `cluster-${clusterName}-workload-proxy`,
    })

    await expect(workloadProxyRow.getByText('Applied')).toBeVisible({
      timeout: milliseconds({ seconds: 30 }),
    })

    await expect(workloadProxyRow.getByText('Full')).toBeVisible()
    await expect(workloadProxyRow.getByText('4/4 in sync')).toBeVisible()
  })

  await test.step('Add service via k8s manifests', async () => {
    const rawManifest = await fs.readFile(new URL('./k8s_manifest.yaml', import.meta.url), 'utf8')
    const parsedManifest = load(rawManifest) as { metadata: { labels?: Record<string, string> } }

    parsedManifest.metadata.labels = {
      ['omni.sidero.dev/cluster']: clusterName,
    }

    const manifestPath = testInfo.outputPath('k8s_manifest.yaml')
    await fs.writeFile(manifestPath, dump(parsedManifest))
    await testInfo.attach('k8s_manifest.yaml', {
      path: manifestPath,
      contentType: 'application/yaml',
    })

    const { stderr } = await omnictl(['apply', '-f', manifestPath])
    expect(stderr.trim()).toHaveLength(0)
  })

  const nginxRow = page.getByRole('row', { name: 'nginx-service' })

  await test.step('Verify nginx service manifest created', async () => {
    await expect(nginxRow.getByText('Progressing')).toBeVisible()
    await expect(nginxRow.getByText('One-Time')).toBeVisible()
    await expect(nginxRow.getByText('0/2 in sync')).toBeVisible()
  })

  await expect(async () => {
    await expect(nginxRow.getByText('Applied')).toBeVisible()
    await expect(nginxRow.getByText('One-Time')).toBeVisible()
    await expect(nginxRow.getByText('2/2 in sync')).toBeVisible()

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

  await test.step('Verify individual nginx service manifests', async () => {
    await expect(nginxRow.getByRole('table')).toBeHidden()
    await nginxRow.getByRole('button', { name: 'Expand details' }).click()
    await expect(nginxRow.getByRole('table')).toBeVisible()

    const deploymentRow = nginxRow.getByRole('row', { name: 'Deployment/default/nginx' })
    const serviceRow = nginxRow.getByRole('row', { name: 'Service/default/nginx' })

    await expect(deploymentRow.getByRole('cell', { name: 'Deployment', exact: true })).toBeVisible()
    await expect(deploymentRow.getByRole('cell', { name: 'nginx', exact: true })).toBeVisible()
    await expect(deploymentRow.getByRole('cell', { name: 'default', exact: true })).toBeVisible()
    await expect(deploymentRow.getByRole('cell', { name: 'Applied', exact: true })).toBeVisible()

    await expect(serviceRow.getByRole('cell', { name: 'Service', exact: true })).toBeVisible()
    await expect(serviceRow.getByRole('cell', { name: 'nginx', exact: true })).toBeVisible()
    await expect(serviceRow.getByRole('cell', { name: 'default', exact: true })).toBeVisible()
    await expect(serviceRow.getByRole('cell', { name: 'Applied', exact: true })).toBeVisible()

    await nginxRow.getByRole('button', { name: 'Collapse details' }).click()
    await expect(nginxRow.getByRole('table')).toBeHidden()
  })
})

test('kubespan', async ({ page }, testInfo) => {
  await test.step('Visit create cluster patch page', async () => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Clusters' }).click()

    await expect(page).toHaveURL('/clusters')
    await expect(page.getByRole('heading', { name: 'Clusters', exact: true })).toBeVisible()

    await page.getByRole('link', { name: clusterName, exact: true }).click()
    await expect(page.getByRole('heading', { name: clusterName, exact: true })).toBeVisible()

    await page.getByRole('main').getByRole('link', { name: 'Config Patches' }).click()
    await page.getByRole('link', { name: 'Create Patch' }).click()
    await expect(page.getByRole('heading', { name: 'Create Patch', exact: true })).toBeVisible()
  })

  await test.step('Add kubespan patch', async () => {
    const kubespanPatch = await fs.readFile(
      new URL('./kubespan_patch.yaml', import.meta.url),
      'utf8',
    )
    await testInfo.attach('kubespan_patch.yaml', {
      body: kubespanPatch,
      contentType: 'application/yaml',
    })

    await page.evaluate((text) => navigator.clipboard.writeText(text), kubespanPatch)
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(kubespanPatch)

    await page
      .getByRole('textbox', { name: 'Editor content' })
      .press(`${os.platform() === 'darwin' ? 'Meta' : 'Control'}+v`)
    await expect(page.getByText('kind: KubeSpanConfig')).toBeVisible()

    await page.getByRole('button', { name: 'Save' }).click()
  })

  await test.step('Visit kubespan page', async () => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Clusters' }).click()

    const servicesList = page.getByRole('region', { name: 'Services' })

    await page.getByRole('region', { name: 'Control Planes' }).getByRole('link').last().click()
    await expect(servicesList.getByRole('link', { name: 'etcd' })).toBeVisible()
    await page.getByRole('tab', { name: 'KubeSpan', exact: true }).click()
  })

  await expect
    .soft(page.getByRole('heading', { name: 'KubeSpan status', exact: true }))
    .toBeVisible()
  await expect.soft(page.getByText('Total Nodes: 2')).toBeVisible()
  await expect.soft(page.getByText('Incoming / Outgoing traffic')).toBeVisible()
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
    await page.getByRole('tab', { name: 'Logs', exact: true }).click()

    await page.getByPlaceholder('Search').fill('[talos] [initramfs] booting Talos')
    await expect
      .soft(page.getByText(/\[talos\] \[initramfs\] booting Talos v\d\.\d+\.\d+/).first())
      .toBeVisible()

    await page.getByPlaceholder('Search').clear()
    await expect
      .soft(page.getByText(/\[talos\] \[initramfs\] booting Talos v\d\.\d+\.\d+/))
      .toBeHidden()
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
    await expect(page.getByText('+variables:')).toBeVisible()

    // This asserts that the virtualisation is working
    await expect(page.getByText('WORKER_THREAD_COUNT')).toBeHidden()
    await page.getByRole('textbox', { name: 'Search:' }).fill('WORKER_THREAD_COUNT')
    await expect(page.getByText('WORKER_THREAD_COUNT')).toBeVisible()
  })

  await test.step('Validate patches tab', async () => {
    await page.getByRole('tab', { name: 'Patches', exact: true }).click()
    await expect(page.getByText('This cluster is managed using cluster templates.')).toBeVisible()
    await expect(page.getByText(`Cluster Machine: ${cpMachineName}`)).toBeVisible()
    await expect(page.getByText(/400-cm-\w+/).first()).toBeVisible()
    await expect(page.getByText('User defined patch').first()).toBeVisible()
  })

  await test.step('Validate kubespan tab', async () => {
    await page.getByRole('tab', { name: 'KubeSpan', exact: true }).click()
  })

  await test.step('Validate disks tab', async () => {
    await page.getByRole('tab', { name: 'Disks', exact: true }).click()

    const row = page.getByRole('region', { name: 'vda' }).getByRole('row', { name: 'EPHEMERAL' })

    await expect(row).toBeVisible()
    await expect(row.getByText('xfs')).toBeVisible()
    await expect(row.getByText('disabled')).toBeVisible()
  })

  await test.step('Validate devices tab', async () => {
    await page.getByRole('tab', { name: 'Devices', exact: true }).click()
    await expect(page.getByText('Ethernet controller')).toBeVisible()
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

test('cluster sidebar pages', async ({ page }) => {
  await test.step('Visit cluster overview', async () => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Clusters' }).click()

    await expect(page).toHaveURL('/clusters')
    await expect(page.getByRole('heading', { name: 'Clusters', exact: true })).toBeVisible()

    await page.getByRole('link', { name: clusterName, exact: true }).click()
    await expect(page.getByRole('heading', { name: clusterName, exact: true })).toBeVisible()
  })

  await test.step('Validate cluster overview', async () => {
    await expect(page.getByRole('heading', { name: clusterName })).toBeVisible()

    const cpuChart = page.getByRole('figure', { name: 'CPU' })

    await expect(cpuChart.getByLabel('Total')).toHaveText(/\d+\.\d{2}/)
    await expect(cpuChart.getByLabel('Requests')).toHaveText(/\d+\.\d{2}/)
    await expect(cpuChart.getByLabel('Limits')).toHaveText(/\d+\.\d{2}/)

    const podsChart = page.getByRole('figure', { name: 'Pods' })

    await expect(podsChart.getByLabel('Total')).toHaveText(/\d+/)
    await expect(podsChart.getByLabel('Requests')).toHaveText(/\d+/)

    const memoryChart = page.getByRole('figure', { name: 'Memory' })

    await expect(memoryChart.getByLabel('Total')).toHaveText(/\d+\.\d{2} GB/)
    await expect(memoryChart.getByLabel('Requests')).toHaveText(/\d+\.\d{2} GB/)
    await expect(memoryChart.getByLabel('Limits')).toHaveText(/\d+\.\d{2} MB/)

    const storageChart = page.getByRole('figure', { name: 'Ephemeral Storage' })

    await expect(storageChart.getByLabel('Total')).toHaveText(/\d+\.\d{2} GB/)
    await expect(storageChart.getByLabel('Requests')).toHaveText('0 Bytes')
    await expect(storageChart.getByLabel('Limits')).toHaveText('0 Bytes')

    await expect(page.getByText('Managed using cluster')).toBeVisible()
  })

  await test.step('Validate cluster nodes', async () => {
    await page.getByRole('link', { name: 'Nodes' }).click()

    await expect(page.getByRole('heading', { name: 'All Nodes' })).toBeVisible()
    await expect(page.getByText(cpMachineName)).toBeVisible()
    await expect(page.getByText('control-plane')).toBeVisible()
  })

  await test.step('Validate cluster pods', async () => {
    await page.getByRole('link', { name: 'Pods' }).click()

    await expect(page.getByRole('heading', { name: 'All Pods' })).toBeVisible()
    await expect(page.getByText('kube-system')).not.toHaveCount(0)
  })

  await test.step('Validate cluster config patches', async () => {
    await page.getByRole('link', { name: 'Config Patches' }).click()
    await expect(
      page.getByRole('heading', { name: `Cluster ${clusterName} Config Patches` }),
    ).toBeVisible()

    await expect(page.getByText('This cluster is managed using cluster templates.')).toBeVisible()
    await expect(page.getByText(`Cluster Machine: ${cpMachineName}`)).toBeVisible()
    await expect(page.getByText(/400-cm-\w+/).first()).toBeVisible()
    await expect(page.getByText('User defined patch').first()).toBeVisible()
  })

  await test.step('Validate cluster bootstrap manifests', async () => {
    await page.getByRole('link', { name: 'Bootstrap Manifests' }).click()

    await expect(
      page.getByRole('heading', { name: `Bootstrap Manifest Sync for ${clusterName}` }),
    ).toBeVisible()
    await expect(page.getByText('Manifest')).not.toHaveCount(0)
  })

  await test.step('Validate backups', async () => {
    await page.getByRole('link', { name: 'Backups' }).click()

    await expect(page.getByRole('heading', { name: 'Control Plane Backups' })).toBeVisible()
    await expect(
      page.getByRole('heading', { name: 'The backups storage is not properly configured' }),
    ).toBeVisible()
  })
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
