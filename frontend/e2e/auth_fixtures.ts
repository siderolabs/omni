// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test as base } from '@playwright/test'

interface AuthFixtures {
  /**
   * We do not use shared auth state because it is incompatible with
   * our setup. Playwright does support storing indexedDB, but it
   * serialises the contents to JSON and does not serialise webcrypto correctly.
   * Even if it did, our keys are unexportable and therefore unserialisable by design.
   */
  auth: void
}

const test = base.extend<AuthFixtures>({
  auth: [
    async ({ page }, use) => {
      if (!process.env.AUTH_USERNAME) throw new Error('username is not set')
      if (!process.env.AUTH_PASSWORD) throw new Error('password is not set')

      await page.goto('/')

      await page.getByRole('textbox', { name: 'Email address' }).fill(process.env.AUTH_USERNAME)
      await page.getByRole('textbox', { name: 'Password' }).fill(process.env.AUTH_PASSWORD)
      await page.getByRole('button', { name: 'Continue', exact: true }).click()

      await page.getByRole('button', { name: 'Log In' }).click()
      await page.getByRole('heading', { name: 'Home' }).waitFor()

      await use()
    },
    { auto: true },
  ],
})

export { expect, test }
