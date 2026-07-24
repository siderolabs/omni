// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import fs from 'node:fs/promises'
import os from 'node:os'

import { faker } from '@faker-js/faker'
import { milliseconds } from 'date-fns'
import { loadAll } from 'js-yaml'

import { expect, test } from './cluster_fixtures'

test.describe.configure({ mode: 'parallel' })

test('View all clusters', async ({ cluster, page }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'Clusters' }).click()

  await expect(page).toHaveURL('/clusters')
  await expect(page.getByRole('heading', { name: 'Clusters', exact: true })).toBeVisible()

  await expect(page.getByText(cluster.name, { exact: true })).toBeVisible()
})

test('Create cluster using machine classes', async ({ page }) => {
  test.setTimeout(milliseconds({ minutes: 3 }))

  const clusterName = `e2e-cluster-${faker.string.alphanumeric(8)}`

  await test.step('Visit clusters page', async () => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Clusters' }).click()

    await expect(page).toHaveURL('/clusters')
    await expect(page.getByRole('heading', { name: 'Clusters', exact: true })).toBeVisible()
  })

  await test.step('Create cluster', async () => {
    await page.getByRole('link', { name: 'Create Cluster' }).click()

    // There is some code to put a default value in the input which we must wait for to prevent being overridden
    await expect(page.getByRole('textbox', { name: 'Cluster Name:' })).toHaveValue(/^talos-default/)
    await page.getByRole('textbox', { name: 'Cluster Name' }).fill(clusterName)

    const controlPlanes = page.getByRole('listitem', { name: 'control planes' })
    const workers = page.getByRole('listitem', { name: 'main worker pool' })

    await controlPlanes.getByRole('button', { name: 'Machine Class' }).click()
    await controlPlanes.getByLabel('Size').fill('1')

    await workers.getByRole('button', { name: 'Machine Class' }).click()
    await workers.getByLabel('Size').fill('2')

    await page.getByRole('button', { name: 'Create Cluster' }).click()
  })

  await expect(async () => {
    await expect(page.getByTestId('machine-set-phase-name').getByText('Running')).toHaveCount(2)
    await expect(page.getByTestId('cluster-machine-stage-name').getByText('Running')).toHaveCount(3)
  }, 'Wait for cluster to be running').toPass({
    intervals: [milliseconds({ seconds: 5 })],
    timeout: milliseconds({ minutes: 1 }),
  })

  await test.step('Destroy cluster', async () => {
    await page.getByRole('button', { name: 'Destroy Cluster' }).click()
    await page.getByRole('button', { name: 'Destroy', exact: true }).click()

    await expect(page.getByText(`The Cluster ${clusterName} is tearing down`)).toBeVisible()
    await expect(page.getByText('Cluster Not Found')).toBeVisible({
      timeout: milliseconds({ minutes: 1 }),
    })
  })
})

test('Scale cluster using machine classes', async ({ omnictl, cluster, page }, testInfo) => {
  test.setTimeout(milliseconds({ minutes: 2 }))

  await test.step('Visit clusters page', async () => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Clusters' }).click()

    await expect(page).toHaveURL('/clusters')
    await expect(page.getByRole('heading', { name: 'Clusters', exact: true })).toBeVisible()
  })

  await expect(
    page
      .getByRole('listitem', { name: cluster.name })
      .getByTestId('cluster-machine-stage-name')
      .getByText('Running'),
    'Assert that cluster only has 3 machines',
  ).toHaveCount(3)

  interface Resource {
    metadata: {
      id: string
      created: Date
    }
  }

  let machineCreatedMap: Map<string, Date>

  // Regression test for #2065 to ensure existing machines are not destroyed during scaling
  await test.step('Check creation times of existing machines', async () => {
    const { stdout: yaml } = await omnictl([
      'get',
      'ClusterMachine',
      '-l',
      `omni.sidero.dev/cluster=${cluster.name}`,
      '-oyaml',
    ])

    const resources = loadAll(yaml) as Resource[]
    await testInfo.attach('resources-before.json', {
      body: JSON.stringify(resources),
      contentType: 'application/json',
    })

    machineCreatedMap = resources.reduce(
      (prev, curr) => prev.set(curr.metadata.id, curr.metadata.created),
      new Map<string, Date>(),
    )
  })

  await test.step('Scale cluster to 5 machines', async () => {
    await page.getByRole('link', { name: cluster.name }).click()
    await page.getByRole('link', { name: 'Cluster Scaling' }).click()

    await expect(page.getByText('Machine Sets', { exact: true })).toBeVisible()

    await page
      .getByRole('listitem', { name: `${cluster.name}-workers` })
      .getByLabel('Size')
      .fill('4')

    await page.getByRole('button', { name: 'Update' }).click()
  })

  await expect(
    page.getByRole('region', { name: 'Workers' }).getByTestId('machine-set-phase-name'),
  ).toHaveText('Scaling Up')

  await expect(async () => {
    await expect(page.getByTestId('machine-set-phase-name').getByText('Running')).toHaveCount(2)
    await expect(page.getByTestId('cluster-machine-stage-name').getByText('Running')).toHaveCount(5)
  }, 'Wait for scaling to complete').toPass({
    intervals: [milliseconds({ seconds: 5 })],
    timeout: milliseconds({ minutes: 1 }),
  })

  await test.step('Assert no existing machines were re-created', async () => {
    const { stdout: yaml } = await omnictl([
      'get',
      'ClusterMachine',
      '-l',
      `omni.sidero.dev/cluster=${cluster.name}`,
      '-oyaml',
    ])

    const resources = loadAll(yaml) as Resource[]
    await testInfo.attach('resources-after.json', {
      body: JSON.stringify(resources),
      contentType: 'application/json',
    })

    const prevMachinesUntouched = resources
      .filter((r) => machineCreatedMap.has(r.metadata.id))
      .every(
        (r) => machineCreatedMap.get(r.metadata.id)?.valueOf() === r.metadata.created.valueOf(),
      )

    expect(prevMachinesUntouched, 'Previous machines were not recreated').toBeTruthy()
  })
})

test('Manage patches', async ({ cluster, page }, testInfo) => {
  await test.step('Visit cluster page', async () => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Clusters' }).click()

    await expect(page).toHaveURL('/clusters')
    await expect(page.getByRole('heading', { name: 'Clusters', exact: true })).toBeVisible()
    await page.getByRole('link', { name: cluster.name }).click()
  })

  await test.step('Add a cluster patch', async () => {
    await page.getByRole('link', { name: 'Config Patches' }).click()
    await page.getByRole('link', { name: 'Create Patch' }).click()

    const envPatch = await fs.readFile(
      new URL('../common/patches/env_config_patch.yaml', import.meta.url),
      'utf8',
    )
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

    await page.getByRole('textbox', { name: 'Name' }).fill('My favourite patch')
    await page.getByRole('textbox', { name: 'Description' }).fill('A patch for all to remember')

    await page.getByRole('button', { name: 'Save' }).click()
  })

  await test.step('Disable / Enable a patch', async () => {
    await page.getByLabel('patch actions').click()
    await page.getByRole('menuitem', { name: 'Disable' }).click()

    await expect(page.getByText('Disabled')).toBeVisible()

    await page.getByLabel('patch actions').click()
    await page.getByRole('menuitem', { name: 'Enable' }).click()

    await expect(page.getByText('Disabled')).toBeHidden()
  })

  await test.step('Delete a patch', async () => {
    await expect(page.getByText('My favourite patch')).toBeVisible()

    await page.getByLabel('patch actions').click()
    await page.getByRole('menuitem', { name: 'Delete' }).click()

    await expect(page.getByText('Please confirm the action')).toBeVisible()
    await page.getByRole('button', { name: 'Destroy' }).click()

    await expect(page.getByText('My favourite patch')).toBeHidden()
  })
})
