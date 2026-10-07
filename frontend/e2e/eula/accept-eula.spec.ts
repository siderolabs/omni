// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, type Page, test } from '@playwright/test'
import { milliseconds } from 'date-fns'

// This project runs before every other one, against dex, Keycloak or Auth0 depending on the run,
// so it matches whichever login form it lands on. See hack/test/e2e-*.sh.
const signIn = async (page: Page) => {
  expect(process.env.AUTH_USERNAME).toBeTruthy()
  expect(process.env.AUTH_PASSWORD).toBeTruthy()

  // the page load can be flaky, so retry until the form shows
  await expect(async () => {
    await page.goto('/')
    await expect(
      page.getByRole('heading', { name: /Log in to Your Account|Sign in to your account|Welcome/ }),
    ).toBeVisible()
  }, 'Navigate to the login page').toPass()

  // Auth0 opens on the signup form
  const logIn = page.getByRole('link', { name: 'Log in' })

  if (await logIn.isVisible()) {
    await logIn.click()
    await expect(page.getByText("Don't have an account?")).toBeVisible()
  }

  await page.getByRole('textbox', { name: /email/i }).fill(process.env.AUTH_USERNAME!)
  await page.getByRole('textbox', { name: 'Password' }).fill(process.env.AUTH_PASSWORD!)
  await page.getByRole('button', { name: /^(Login|Sign In|Continue)$/ }).click()
}

test('accept EULA', async ({ page }) => {
  await signIn(page)

  await expect(page.getByRole('heading', { name: 'End User License Agreement' })).toBeVisible({
    timeout: milliseconds({ seconds: 15 }),
  })
  await expect(page.getByRole('button', { name: 'Accept' })).toBeDisabled()

  await page.getByRole('textbox', { name: 'Full Name:' }).fill('Test User')
  await page.getByRole('textbox', { name: 'Email Address:' }).fill('test-user@siderolabs.com')
  await page.getByRole('checkbox', { name: 'I have read and agree to the' }).check()

  await page.getByRole('button', { name: 'Accept' }).click()

  await expect(page.getByRole('heading', { name: 'Home' }), 'Should redirect to home').toBeVisible({
    timeout: milliseconds({ seconds: 15 }),
  })
})
