// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { readFile } from 'node:fs/promises'

import { faker } from '@faker-js/faker'
import { loadAll } from 'js-yaml'

import { expect, test } from '../auth_fixtures'

const DEFAULT_TOKEN = 'testonly'
const HIDDEN_TOKEN = /•+/
const NEW_TOKEN = faker.string.alphanumeric(16)

test.describe.configure({ mode: 'serial' })

test('Join tokens list read functionality', async ({ page }, testInfo) => {
  await page.goto('/')

  await test.step('Visit join tokens page', async () => {
    await page.goto('/')
    await page.getByRole('button', { name: 'Machine Management' }).click()
    await page.getByRole('link', { name: 'Join Tokens' }).click()
    await expect(page.getByRole('heading', { name: 'Machine Join Tokens' })).toBeVisible()
  })

  const row = page.getByRole('row', { name: 'initial token' })

  await test.step('List is not empty', async () => {
    await expect(row.getByText('initial token')).toBeVisible()
    await expect(row.getByText('Default')).toBeVisible()
    await expect(row.getByText('Active')).toBeVisible()
    await expect(row.getByText('Never')).toBeVisible()
  })

  await test.step('Tokens are masked by default', async () => {
    await expect(row.getByText(HIDDEN_TOKEN)).toBeVisible()
    await expect(row.getByText(DEFAULT_TOKEN)).toBeHidden()
  })

  await test.step('Tokens can be unmasked & remasked', async () => {
    await row.getByText(HIDDEN_TOKEN).click()

    await expect(row.getByText(HIDDEN_TOKEN)).toBeHidden()
    await expect(row.getByText(DEFAULT_TOKEN)).toBeVisible()

    await row.getByText(DEFAULT_TOKEN).click()

    await expect(row.getByText(HIDDEN_TOKEN)).toBeVisible()
    await expect(row.getByText(DEFAULT_TOKEN)).toBeHidden()
  })

  await test.step('Basic token actions work', async () => {
    await row.getByLabel('token actions').click()
    await page.getByRole('menuitem', { name: 'Copy Token' }).click()

    await expect
      .poll(async () => await page.evaluate(() => navigator.clipboard.readText()))
      .toBe(DEFAULT_TOKEN)

    await row.getByLabel('token actions').click()
    await page.getByRole('menuitem', { name: 'Copy Kernel Params' }).click()

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
        'siderolink.api': expect.stringMatching(
          new RegExp(`grpc://[\\w.-]+(:\\d+)?\\?jointoken=${DEFAULT_TOKEN}`),
        ),
        'talos.events.sink': expect.stringMatching(/\[fdae:41e4:649b:9303::1\]:\d+/),
        'talos.logging.kernel': expect.stringMatching(/tcp:\/\/\[fdae:41e4:649b:9303::1\]:\d+/),
      })

    await row.getByLabel('token actions').click()

    const [downloadConfig] = await Promise.all([
      page.waitForEvent('download'),
      await page.getByRole('menuitem', { name: 'Download Machine Join Config' }).click(),
    ])

    const omniconfigPath = testInfo.outputPath(downloadConfig.suggestedFilename())
    await downloadConfig.saveAs(omniconfigPath)
    await testInfo.attach(downloadConfig.suggestedFilename(), { path: omniconfigPath })

    const documents = loadAll(await readFile(omniconfigPath, 'utf-8'))

    expect(documents).toEqual([
      {
        apiUrl: expect.stringMatching(
          new RegExp(`grpc://[\\w.-]+(:\\d+)?\\?jointoken=${DEFAULT_TOKEN}`),
        ),
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
})

test('Create new join token', async ({ page }) => {
  await page.goto('/')

  await test.step('Visit join tokens page', async () => {
    await page.goto('/')
    await page.getByRole('button', { name: 'Machine Management' }).click()
    await page.getByRole('link', { name: 'Join Tokens' }).click()
    await expect(page.getByRole('heading', { name: 'Machine Join Tokens' })).toBeVisible()
  })

  await test.step('Open create modal', async () => {
    await page.getByRole('button', { name: 'Create Join Token' }).click()
    await expect(page.getByRole('heading', { name: 'Create Join Token' })).toBeVisible()
  })

  await test.step('Submit form', async () => {
    await page.getByRole('button', { name: 'Limited' }).click()
    await page.getByRole('textbox', { name: 'Name' }).fill(NEW_TOKEN)
    await page.getByRole('spinbutton', { name: 'Expiration Days' }).fill('14')
    await page.getByRole('button', { name: 'Create Join Token' }).click()
  })

  const row = page.getByRole('row', { name: NEW_TOKEN })

  await test.step('List contains new token', async () => {
    await expect(row.getByText(NEW_TOKEN)).toBeVisible()
    await expect(row.getByText('Default')).toBeHidden()
    await expect(row.getByText('Active')).toBeVisible()
    await expect(row.getByText('in 13 days')).toBeVisible()
    await expect(row.getByText(HIDDEN_TOKEN)).toBeVisible()
  })
})

test('Make token default', async ({ page }) => {
  await page.goto('/')

  await test.step('Visit join tokens page', async () => {
    await page.goto('/')
    await page.getByRole('button', { name: 'Machine Management' }).click()
    await page.getByRole('link', { name: 'Join Tokens' }).click()
    await expect(page.getByRole('heading', { name: 'Machine Join Tokens' })).toBeVisible()
  })

  const row = page.getByRole('row', { name: NEW_TOKEN })

  await test.step('Make token default', async () => {
    await expect(row.getByText('Default')).toBeHidden()

    await row.getByLabel('token actions').click()
    await page.getByRole('menuitem', { name: 'Make Default' }).click()

    await expect(row.getByText('Default')).toBeVisible()
  })
})

test('Revoke join token', async ({ page }) => {
  await page.goto('/')

  await test.step('Visit join tokens page', async () => {
    await page.goto('/')
    await page.getByRole('button', { name: 'Machine Management' }).click()
    await page.getByRole('link', { name: 'Join Tokens' }).click()
    await expect(page.getByRole('heading', { name: 'Machine Join Tokens' })).toBeVisible()
  })

  const row = page.getByRole('row', { name: NEW_TOKEN })

  await test.step('Revoke token', async () => {
    await row.getByLabel('token actions').click()
    await page.getByRole('menuitem', { name: 'Revoke' }).click()

    await expect(page.getByRole('heading', { name: /Revoke the token \w+ ?/ })).toBeVisible()
    await expect(page.getByText('The token can be safely revoked/deleted')).toBeVisible()

    await page.getByRole('button', { name: 'Revoke' }).click()
  })

  await test.step('Assert revocation', async () => {
    await expect(row.getByText('Active')).toBeHidden()
    await expect(row.getByText('Revoked')).toBeVisible()
  })

  await test.step('Unrevoke', async () => {
    await row.getByLabel('token actions').click()
    await page.getByRole('menuitem', { name: 'Unrevoke' }).click()
  })

  await test.step('Assert unrevocation', async () => {
    await expect(row.getByText('Active')).toBeVisible()
    await expect(row.getByText('Revoked')).toBeHidden()
  })
})

test('Delete join token', async ({ page }) => {
  await page.goto('/')

  await test.step('Visit join tokens page', async () => {
    await page.goto('/')
    await page.getByRole('button', { name: 'Machine Management' }).click()
    await page.getByRole('link', { name: 'Join Tokens' }).click()
    await expect(page.getByRole('heading', { name: 'Machine Join Tokens' })).toBeVisible()
  })

  const row = page.getByRole('row', { name: NEW_TOKEN })

  await test.step('Revoke token', async () => {
    await row.getByLabel('token actions').click()
    await page.getByRole('menuitem', { name: 'Revoke' }).click()

    await expect(page.getByRole('heading', { name: /Revoke the token \w+ ?/ })).toBeVisible()
    await expect(page.getByText('The token can be safely revoked/deleted')).toBeVisible()

    await page.getByRole('button', { name: 'Revoke' }).click()
  })

  await test.step('Get blocked trying to delete the default token', async () => {
    await row.getByLabel('token actions').click()
    await page.getByRole('menuitem', { name: 'Delete' }).click()
    await page.getByRole('button', { name: 'Delete' }).click()

    await expect(page.getByText('deleting default join token is not possible')).toBeVisible()

    await page.getByRole('row', { name: 'initial token' }).getByLabel('token actions').click()
    await page.getByRole('menuitem', { name: 'Make Default' }).click()
  })

  await test.step('Delete token', async () => {
    await row.getByLabel('token actions').click()
    await page.getByRole('menuitem', { name: 'Delete' }).click()

    await expect(page.getByRole('heading', { name: /Delete the token \w+ ?/ })).toBeVisible()
    await expect(page.getByText('The token can be safely revoked/deleted')).toBeVisible()
    await expect(
      page.getByText('This action CANNOT be undone. This will permanently delete the Join Token.'),
    ).toBeVisible()

    await page.getByRole('button', { name: 'Delete' }).click()
  })

  await expect(page.getByRole('heading', { name: 'Machine Join Tokens' })).toBeVisible()
  await expect(row).toBeHidden()
})
