// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from '@playwright/test'
import { milliseconds } from 'date-fns'

test('Auth flow', async ({ page }) => {
  expect(process.env.AUTH_USERNAME).toBeTruthy()
  expect(process.env.AUTH_PASSWORD).toBeTruthy()

  await test.step('Navigate to Keycloak login page', async () => {
    // Navigating to Omni redirects through the backend /login handler to the Keycloak
    // login form. The page load can be flaky, so retry until the form shows.
    await expect(async () => {
      await page.goto('/')
      await expect(page.getByRole('heading', { name: 'Sign in to your account' })).toBeVisible()
    }, 'Navigate to Keycloak login page').toPass()
  })

  await test.step('Login', async () => {
    await page.getByRole('textbox', { name: 'Username or email' }).fill(process.env.AUTH_USERNAME!)
    await page.getByRole('textbox', { name: 'Password' }).fill(process.env.AUTH_PASSWORD!)
    await page.getByRole('button', { name: 'Sign In' }).click()
  })

  await expect(page.getByRole('heading', { name: 'Home' }), 'Should redirect to home').toBeVisible({
    timeout: milliseconds({ seconds: 15 }),
  })

  await test.step('Identity comes from the SAML assertion', async () => {
    // Omni reads the email from the X500 email attribute and builds the full name from
    // givenName and surname, so these two cover the attribute mapping and not just the
    // fact that a session exists.
    await expect(page.getByText(process.env.AUTH_USERNAME!, { exact: true })).toBeVisible()
    await expect(page.getByText('Test User', { exact: true })).toBeVisible()
  })

  await test.step('Logout', async () => {
    // Keycloak answering the LogoutRequest is what proves Single Logout ran, and it has to be watched for
    // rather than inferred from the login form: every login now carries ForceAuthn, so that form appears
    // whether or not the IdP session was ended.
    const logoutResponse = page.waitForRequest((request) => request.url().includes('/saml/slo'))

    await page.getByRole('button', { name: 'user actions' }).click()
    await page.getByRole('menuitem', { name: 'Log Out' }).click()

    await logoutResponse

    await expect(page.getByRole('heading', { name: 'Sign in to your account' })).toBeVisible()
  })
})
