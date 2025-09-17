// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { readFile, stat } from 'node:fs/promises'

import { expect, test } from '@playwright/test'
import * as yaml from 'yaml'

test.describe.configure({ mode: 'parallel' })

test('Has expected title', async ({ page }) => {
  await page.goto('/')

  await expect(page).toHaveTitle('Omni - default')
})

test('Download installation media', async ({ page }, testInfo) => {
  test.slow()

  await page.goto('/')

  await page.getByRole('button', { name: 'Download Installation Media' }).click()
  await page.getByText('hello-world-service').click()
  await page.getByRole('button', { name: 'Download', exact: true }).click()

  const [download] = await Promise.all([
    page.waitForEvent('download'),
    expect(page.getByText('Generating Image')).toBeVisible(),
  ])

  await expect(page.getByRole('heading', { name: 'Home' })).toBeVisible()

  const filePath = testInfo.outputPath(download.suggestedFilename())
  await download.saveAs(filePath)

  const { size } = await stat(filePath)

  expect(size).toBeGreaterThan(50 * 1024 * 1024)
})

test('Download machine join config', async ({ page }, testInfo) => {
  await page.goto('/')

  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('button', { name: 'Download Machine Join Config' }).click(),
  ])

  const filePath = testInfo.outputPath(download.suggestedFilename())
  await download.saveAs(filePath)
  await testInfo.attach(download.suggestedFilename(), { path: filePath })

  const documents = yaml.parseAllDocuments(await readFile(filePath, 'utf-8')).map((d) => d.toJS())

  expect(documents).toEqual([
    {
      apiUrl: expect.stringMatching(/grpc:\/\/[\w\.-]+(:\d+)?\?jointoken=\w+/),
      apiVersion: 'v1alpha1',
      kind: 'SideroLinkConfig',
    },
    {
      apiVersion: 'v1alpha1',
      endpoint: expect.stringMatching(/\[fdae:41e4:649b:9303::1\]:\d+/),
      kind: 'EventSinkConfig',
    },
    {
      apiVersion: 'v1alpha1',
      kind: 'KmsgLogConfig',
      name: 'omni-kmsg',
      url: expect.stringMatching(/tcp:\/\/\[fdae:41e4:649b:9303::1\]:\d+/),
    },
  ])
})

test('Copying kernel parameters', async ({ page }) => {
  await page.goto('/')

  await page.getByRole('button', { name: 'Copy Kernel Parameters' }).click()

  await expect
    .poll(async () => {
      const handle = await page.evaluateHandle(() => navigator.clipboard.readText())
      const value = await handle.jsonValue()

      return Object.fromEntries(
        value.split(' ').map((s) => {
          const [key, ...value] = s.split('=')
          return [key, value.join('=')]
        }),
      )
    }, 'Expect kernel parameters to be correct')
    .toEqual({
      'siderolink.api': expect.stringMatching(/grpc:\/\/[\w\.-]+(:\d+)?\?jointoken=\w+/),
      'talos.events.sink': expect.stringMatching(/\[fdae:41e4:649b:9303::1\]:\d+/),
      'talos.logging.kernel': expect.stringMatching(/tcp:\/\/\[fdae:41e4:649b:9303::1\]:\d+/),
    })
})

test('Download talosconfig', async ({ page }, testInfo) => {
  await page.goto('/')

  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('button', { name: 'Download talosconfig' }).click(),
  ])

  const filePath = testInfo.outputPath(download.suggestedFilename())
  await download.saveAs(filePath)
  await testInfo.attach(download.suggestedFilename(), { path: filePath })

  const document = yaml.parse(await readFile(filePath, 'utf-8'))

  expect(document).toEqual({
    context: 'default',
    contexts: {
      default: {
        endpoints: [expect.stringMatching(/https:\/\/[\w\.-]+(:\d+)?/)],
        auth: {
          siderov1: {
            identity: process.env.AUTH_USERNAME,
          },
        },
      },
    },
  })
})

test('Download omniconfig', async ({ page }, testInfo) => {
  await page.goto('/')

  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('button', { name: 'Download omniconfig' }).click(),
  ])

  const filePath = testInfo.outputPath(download.suggestedFilename())
  await download.saveAs(filePath)
  await testInfo.attach(download.suggestedFilename(), { path: filePath })

  const document = yaml.parse(await readFile(filePath, 'utf-8'))

  expect(document).toEqual({
    context: 'default',
    contexts: {
      default: {
        url: expect.stringMatching(/https:\/\/[\w\.-]+(:\d+)?/),
        auth: {
          siderov1: {
            identity: process.env.AUTH_USERNAME,
          },
        },
      },
    },
  })
})

test('Download omnictl', async ({ page }, testInfo) => {
  await page.goto('/')

  await page.getByRole('button', { name: 'Download omnictl' }).click()

  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('button', { name: 'Download', exact: true }).click(),
  ])

  await expect(page.getByRole('heading', { name: 'Home' })).toBeVisible()

  const filePath = testInfo.outputPath(download.suggestedFilename())
  await download.saveAs(filePath)

  const { size } = await stat(filePath)

  expect(size).toBeGreaterThan(5 * 1024 * 1024)
})

test('Get audit logs', async ({ page }, testInfo) => {
  await page.goto('/')

  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('button', { name: 'Get audit logs' }).click(),
  ])

  const filePath = testInfo.outputPath(download.suggestedFilename())
  await download.saveAs(filePath)
  await testInfo.attach(download.suggestedFilename(), { path: filePath })

  await expect(readFile(filePath, 'utf-8')).resolves.toContain(
    '"resource_type":"Identities.omni.sidero.dev"',
  )
})

test('View all clusters', async ({ page }) => {
  await page.goto('/')

  await page
    .locator('section')
    .filter({ hasText: 'Recent Clusters' })
    .getByRole('button', { name: 'View All' })
    .click()

  await expect(page).toHaveURL('/clusters')
  await expect(page.getByRole('heading', { name: 'Clusters' })).toBeVisible()
})

test('View all Machines', async ({ page }) => {
  await page.goto('/')

  await page
    .locator('section')
    .filter({ hasText: 'Recent Machines' })
    .getByRole('button', { name: 'View All' })
    .click()

  await expect(page).toHaveURL('/machines')
  await expect(page.getByRole('heading', { name: 'Machines' })).toBeVisible()
})

test('Shows general information', async ({ page }) => {
  await page.goto('/')

  await expect(page.getByText(/^v\d+\.\d+\.\d+/), 'Expect valid backend version').toBeVisible()
  await expect(page.getByText(/grpc:\/\/[\w\.-]+:\d+/), 'Expect valid API endpoint').toBeVisible()
  await expect(page.getByText(/[\w\.-]+:50180/), 'Expect valid WireGuard endpoint').toBeVisible()

  await expect(page.getByText(/•+/), 'Expect join token hidden').toBeVisible()
  await page.getByText(/•+/).click()
  await expect(page.getByText(/•+/), 'Expect join token visible').toBeHidden()
})

test('Opens documentation', async ({ page }) => {
  await page.goto('/')

  const [docsPage] = await Promise.all([
    page.waitForEvent('popup'),
    page.getByRole('link', { name: 'Documentation' }).click(),
  ])

  await expect(docsPage).toHaveURL(/^https:\/\/docs.siderolabs.com/)
  await expect(docsPage.getByRole('heading', { name: 'Omni Documentation' })).toBeVisible()
})
