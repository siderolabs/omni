// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from '@playwright/test'

test('Auth flow', async ({ page }) => {
  expect(process.env.AUTH_USERNAME).toBeTruthy()
  expect(process.env.AUTH_PASSWORD).toBeTruthy()

  await test.step('Navigate to auth0 login page', async () => {
    await page.goto('/')
    await expect(page.getByRole('heading', { name: 'Welcome' })).toBeVisible()
  })

  await test.step('Switch from signup to login', async () => {
    await page.getByRole('link', { name: 'Log in' }).click()
    await expect(page.getByText("Don't have an account?")).toBeVisible()
  })

  await test.step('Login', async () => {
    await page.getByRole('textbox', { name: 'Email address' }).fill(process.env.AUTH_USERNAME!)
    await page.getByRole('textbox', { name: 'Password' }).fill(process.env.AUTH_PASSWORD!)
    await page.getByRole('button', { name: 'Continue', exact: true }).click()
  })

  await expect(page.getByRole('heading', { name: 'Home' }), 'Should redirect to home').toBeVisible()

  await test.step('Logout', async () => {
    await page.getByRole('button', { name: 'user actions' }).click()
    await page.getByRole('menuitem', { name: 'Log Out' }).click()

    await expect(page.getByText('Log in')).toBeVisible()
  })
})
