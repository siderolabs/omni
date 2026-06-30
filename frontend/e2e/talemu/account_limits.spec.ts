// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Page } from '@playwright/test'
import { milliseconds } from 'date-fns'

import { expect, test } from '../omnictl_fixtures'

// These limits must match the values in hack/test/templates/omni-config.yaml.
// Note: the server may already have accounts (e.g. the logged-in test user, auto-created
// service accounts), so tests create accounts until hitting the limit rather than computing
// an exact count.
const maxServiceAccounts = 5
const maxUsers = 5
const saPrefix = 'e2e-limit-sa-'
const userPrefix = 'e2e-limit-user-'

test.describe.configure({ mode: 'serial' })

// ---------------------------------------------------------------------------
// Service account helpers
// ---------------------------------------------------------------------------

/** Delete a service account via the UI. Assumes the page is on /settings/serviceaccounts. */
async function deleteServiceAccountViaUI(page: Page, name: string): Promise<void> {
  const fullID = `${name}@serviceaccount.omni.sidero.dev`

  const row = page.getByRole('row').filter({ hasText: fullID })
  await row.getByRole('button').click()

  await page.getByRole('menuitem', { name: 'Delete Service Account' }).click()

  await page.getByRole('alertdialog').getByRole('button', { name: 'Delete' }).click()
  await expect(page.getByRole('alertdialog')).toBeHidden()
}

// ---------------------------------------------------------------------------
// User helpers
// ---------------------------------------------------------------------------

/** Delete a user via the UI. Assumes the page is on /settings/users. */
async function deleteUserViaUI(page: Page, email: string): Promise<void> {
  const row = page.getByRole('row').filter({ hasText: email })
  await row.getByRole('button').click()

  await page.getByRole('menuitem', { name: 'Delete User' }).click()

  await page.getByRole('alertdialog').getByRole('button', { name: 'Delete' }).click()
  await expect(page.getByRole('alertdialog')).toBeHidden()
}

// ---------------------------------------------------------------------------
// Service account limit tests
// ---------------------------------------------------------------------------

test.describe('Account limits', () => {
  test('service account creation via UI is blocked when limit is reached', async ({ page }) => {
    test.setTimeout(milliseconds({ minutes: 3 }))

    await page.goto('/settings/serviceaccounts')
    await expect(page.getByText('Service Accounts', { exact: true }).first()).toBeVisible()

    // Clean up any leftover e2e service accounts from previous runs.
    const saRows = page.getByRole('row').getByText('@serviceaccount.omni.sidero.dev')
    const saCount = await saRows.count()

    for (let i = saCount - 1; i >= 0; i--) {
      const text = await saRows.nth(i).textContent()
      if (text?.includes(saPrefix)) {
        const name = text.split('@')[0]
        await deleteServiceAccountViaUI(page, name)
      }
    }

    const createdServiceAccounts: string[] = []

    try {
      for (let i = 0; i < maxServiceAccounts; i++) {
        const name = `${saPrefix}ui-${Date.now()}-${i}`

        await page.getByRole('button', { name: 'Create Service Account' }).first().click()
        await expect(page.getByRole('heading', { name: 'Create Service Account' })).toBeVisible()

        await page.getByRole('textbox', { name: 'ID:' }).fill(name)

        await page
          .locator('.modal-window')
          .getByRole('button', { name: 'Create Service Account' })
          .click()

        const result = await Promise.race([
          page
            .getByText('Store the key securely')
            .waitFor()
            .then(() => 'success' as const),
          page
            .locator('[data-sonner-toast][data-type="error"]')
            .waitFor()
            .then(() => 'error' as const),
        ])

        if (result === 'error') {
          await expect(page.locator('[data-sonner-toast][data-type="error"]')).toContainText(
            'maximum number of service accounts',
          )

          await page.locator('.modal-window').getByRole('button', { name: 'close' }).click()

          return
        }

        await page.locator('.modal-window').getByRole('button', { name: 'close' }).click()
        await expect(page.locator('.modal-window')).toBeHidden()

        createdServiceAccounts.push(name)
      }

      await page.getByRole('button', { name: 'Create Service Account' }).first().click()
      await expect(page.getByRole('heading', { name: 'Create Service Account' })).toBeVisible()

      const saName = `${saPrefix}ui-blocked-${Date.now()}`

      await page.getByRole('textbox', { name: 'ID:' }).fill(saName)

      await page
        .locator('.modal-window')
        .getByRole('button', { name: 'Create Service Account' })
        .click()

      await expect(
        page.locator('[data-sonner-toast][data-type="error"]'),
        'Expect error toast about service account limit',
      ).toBeVisible({ timeout: milliseconds({ seconds: 10 }) })

      await expect(page.locator('[data-sonner-toast][data-type="error"]')).toContainText(
        'maximum number of service accounts',
      )
    } finally {
      try {
        if (await page.locator('.modal-window').isVisible()) {
          await page.locator('.modal-window').getByRole('button', { name: 'close' }).click()
        }

        for (const name of createdServiceAccounts) {
          await deleteServiceAccountViaUI(page, name)
        }
      } catch {
        // Page may already be closed on timeout — ignore cleanup errors.
      }
    }
  })

  // ---------------------------------------------------------------------------
  // User limit tests
  // ---------------------------------------------------------------------------

  test('user creation via UI is blocked when limit is reached', async ({ page }) => {
    test.setTimeout(milliseconds({ minutes: 3 }))

    await page.goto('/settings/users')
    await expect(page.getByText('Users', { exact: true }).first()).toBeVisible()

    // Clean up any leftover e2e users from previous runs.
    const userRows = page.getByRole('row').getByText(userPrefix)
    const userCount = await userRows.count()

    for (let i = userCount - 1; i >= 0; i--) {
      const text = await userRows.nth(i).textContent()
      if (text) {
        const email = text.trim()
        await deleteUserViaUI(page, email)
      }
    }

    const createdUsers: string[] = []

    try {
      for (let i = 0; i < maxUsers; i++) {
        const email = `${userPrefix}ui-${Date.now()}-${i}@test.com`

        await page.getByRole('button', { name: 'Add User' }).click()
        await expect(page.getByRole('heading', { name: 'Create User' })).toBeVisible()

        await page.getByRole('textbox', { name: 'User Email:' }).fill(email)

        await page.getByRole('dialog').getByRole('button', { name: 'Create User' }).click()

        // Check if we got an error (limit reached) or success (modal closes).
        const result = await Promise.race([
          page
            .getByRole('dialog')
            .waitFor({ state: 'hidden' })
            .then(() => 'success' as const),
          page
            .locator('[data-sonner-toast][data-type="error"]')
            .waitFor()
            .then(() => 'error' as const),
        ])

        if (result === 'error') {
          await expect(page.locator('[data-sonner-toast][data-type="error"]')).toContainText(
            'maximum number of users',
          )

          await page.getByRole('dialog').getByRole('button', { name: 'close' }).first().click()

          return
        }

        createdUsers.push(email)
      }

      // If all creates succeeded, the next one must fail.
      const email = `${userPrefix}ui-blocked-${Date.now()}@test.com`

      await page.getByRole('button', { name: 'Add User' }).click()
      await expect(page.getByRole('heading', { name: 'Create User' })).toBeVisible()

      await page.getByRole('textbox', { name: 'User Email:' }).fill(email)

      await page.getByRole('dialog').getByRole('button', { name: 'Create User' }).click()

      await expect(
        page.locator('[data-sonner-toast][data-type="error"]'),
        'Expect error toast about user limit',
      ).toBeVisible({ timeout: milliseconds({ seconds: 10 }) })

      await expect(page.locator('[data-sonner-toast][data-type="error"]')).toContainText(
        'maximum number of users',
      )
    } finally {
      try {
        await page.keyboard.press('Escape')
        await expect(page.getByRole('dialog')).toBeHidden()

        for (const email of createdUsers) {
          await deleteUserViaUI(page, email)
        }
      } catch {
        // Page may already be closed on timeout — ignore cleanup errors.
      }
    }
  })
})
