// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from '../auth_fixtures'

test.describe.configure({ mode: 'parallel' })

test('Logout', async ({ page }) => {
  await page.goto('/')

  await page.getByRole('button', { name: 'user actions' }).click()
  await page.getByRole('menuitem', { name: 'Log Out' }).click()

  await expect(page.getByText('Log in')).toBeVisible()
})
