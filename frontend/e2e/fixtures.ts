// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test as base } from '@playwright/test'

interface LoginFixtures {
  login: void
  loginUser: void
  loginCLI: void
}

const test = base.extend<LoginFixtures>({
  login: async ({ page }, use) => {
    if (!process.env.AUTH_USERNAME) {
      throw new Error('username is not set')
    }

    if (!process.env.AUTH_PASSWORD) {
      throw new Error('password is not set')
    }

    await page.goto('/')

    await page.getByRole('textbox', { name: 'Email address' }).fill(process.env.AUTH_USERNAME)
    await page.getByRole('textbox', { name: 'Password' }).fill(process.env.AUTH_PASSWORD)
    await page.getByRole('button', { name: 'Continue', exact: true }).click()

    await use()
  },
  loginUser: async ({ page, login }, use) => {
    await page.getByRole('button', { name: 'Log In' }).click()
    await expect(page.getByRole('heading', { name: 'Home' })).toBeVisible()

    await use()
  },
  loginCLI: async ({ page, login }, use) => {
    await page.getByRole('button', { name: 'Grant Access' }).click()
    await expect(page.getByText('Successfully logged in')).toBeVisible()

    await use()
  },
})

export { expect, test }
