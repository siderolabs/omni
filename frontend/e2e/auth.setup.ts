// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import path from 'node:path'

import { test as setup } from '@playwright/test'

/**
 * See: https://playwright.dev/docs/auth
 */
setup('auth setup', async ({ page }) => {
  if (!process.env.AUTH_USERNAME) throw new Error('username is not set')
  if (!process.env.AUTH_PASSWORD) throw new Error('password is not set')

  await page.goto('/')

  await page.getByRole('textbox', { name: 'Email address' }).fill(process.env.AUTH_USERNAME)
  await page.getByRole('textbox', { name: 'Password' }).fill(process.env.AUTH_PASSWORD)
  await page.getByRole('button', { name: 'Continue', exact: true }).click()

  await page.getByRole('button', { name: 'Log In' }).click()
  await page.getByRole('heading', { name: 'Home' }).waitFor()

  await page.context().storageState({ path: path.join('e2e', '.auth', 'user.json') })
})
