// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Page } from '@playwright/test'

import { expect, test } from '../auth_fixtures'

const NAME = `e2e-sa-${faker.string.alphanumeric({ length: 8, casing: 'lower' })}`
const FULL_ID = `${NAME}@serviceaccount.omni.sidero.dev`

test.describe.configure({ mode: 'serial', retries: 0 })

const getItem = (page: Page) => page.getByRole('listitem', { name: FULL_ID })

async function expandKeys(page: Page) {
  const trigger = getItem(page).getByRole('button', { name: FULL_ID })

  await trigger.click()
  await expect(trigger).toHaveAttribute('aria-expanded', 'true')

  return getItem(page).getByRole('region', { name: 'Public keys' })
}

test.beforeEach(async ({ page }) => {
  await test.step('Visit service accounts page', async () => {
    await page.goto('/settings/serviceaccounts')
    await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible()
    await expect(page.getByText('Service Accounts', { exact: true }).first()).toBeVisible()

    // Other specs create service accounts too, filter so ours is not pushed to another page
    await page.getByRole('textbox').fill(NAME)
  })
})

test('Create service account', async ({ page }) => {
  await test.step('Submit form', async () => {
    await page.getByRole('button', { name: 'Create Service Account' }).click()
    await expect(page.getByRole('heading', { name: 'Create Service Account' })).toBeVisible()

    await page.getByRole('textbox', { name: 'ID:' }).fill(NAME)
    await page.getByRole('dialog').getByRole('button', { name: 'Create Service Account' }).click()
  })

  await test.step('Key is shown', async () => {
    await expect(page.getByText('OMNI_SERVICE_ACCOUNT_KEY')).toBeVisible()
    await expect(page.getByText('Store the key securely')).toBeVisible()

    await page.getByRole('dialog').getByRole('button', { name: 'Close', exact: true }).click()
    await expect(page.getByRole('dialog')).toBeHidden()
  })

  await test.step('List contains new service account', async () => {
    const item = getItem(page)

    await expect(item).toBeVisible()
    await expect(item.getByText('Reader')).toBeVisible()
  })
})

test('List service account keys', async ({ page }) => {
  const keys = await expandKeys(page)

  await expect(keys.getByText('Public Key ID')).toBeVisible()
  await expect(keys.getByRole('button', { name: 'Revoke key' })).toHaveCount(1)
  await expect(keys.getByRole('button', { name: 'Revoke key' })).toBeDisabled()
})

test('Renew service account key', async ({ page }) => {
  await test.step('Generate new key', async () => {
    await getItem(page).getByRole('button', { name: 'service account actions' }).click()
    await page.getByRole('menuitem', { name: 'Renew Key' }).click()

    await expect(page.getByRole('heading', { name: 'Renew Service Account Key' })).toBeVisible()
    await page.getByRole('button', { name: 'Generate New Key' }).click()

    await expect(page.getByText('OMNI_SERVICE_ACCOUNT_KEY')).toBeVisible()

    await page.getByRole('dialog').getByRole('button', { name: 'Close', exact: true }).click()
    await expect(page.getByRole('dialog')).toBeHidden()
  })

  await test.step('Both keys are listed', async () => {
    const keys = await expandKeys(page)

    await expect(keys.getByRole('button', { name: 'Revoke key' })).toHaveCount(2)
  })
})

test('Revoke service account key', async ({ page }) => {
  const keys = await expandKeys(page)
  const revokeButtons = keys.getByRole('button', { name: 'Revoke key' })

  await test.step('Revoke one key', async () => {
    await expect(revokeButtons).toHaveCount(2)
    await revokeButtons.first().click()

    const dialog = page.getByRole('alertdialog')

    await expect(dialog.getByRole('heading', { name: 'Revoke Service Account Key' })).toBeVisible()
    await dialog.getByRole('button', { name: 'Revoke key' }).click()
    await expect(dialog).toBeHidden()
  })

  await test.step('Only one key remains', async () => {
    await expect(revokeButtons).toHaveCount(1)
    await expect(revokeButtons).toBeDisabled()
  })
})

test('Delete service account', async ({ page }) => {
  const item = getItem(page)

  await item.getByRole('button', { name: 'service account actions' }).click()
  await page.getByRole('menuitem', { name: 'Delete Service Account' }).click()

  const dialog = page.getByRole('alertdialog')

  await expect(dialog.getByRole('heading', { name: 'Delete Service Account' })).toBeVisible()
  await dialog.getByRole('button', { name: 'Delete' }).click()
  await expect(dialog).toBeHidden()

  await expect(item).toBeHidden()
})
