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

test('Key expiration without existing auth session', async ({ page }) => {
  await page.goto('/')

  await expect(page.getByRole('heading', { name: 'Home' })).toBeVisible()

  // Invalidate keys
  await page.evaluate(() => {
    localStorage.setItem('keyExpirationTime', new Date(0).toISOString())
  })
  // Clear auth0 cookies
  await page.context().clearCookies()
  await page.goto('/')

  await expect(page.getByText('Log in')).toBeVisible()
})

test('Key expiration with existing auth session', async ({ page }) => {
  await page.goto('/')

  await expect(page.getByRole('heading', { name: 'Home' })).toBeVisible()

  // Invalidate keys
  await page.evaluate(() => {
    localStorage.setItem('keyExpirationTime', new Date(0).toISOString())
  })
  await page.goto('/')

  // Need this to make sure getByText polling doesn't miss the auth page
  await page.waitForURL('**/authenticate**')

  await expect(page.getByText('Authenticate UI Access')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Home' })).toBeVisible()
})

test('Escape invalid key state', async ({ page }) => {
  await page.goto('/')

  await expect(page.getByRole('heading', { name: 'Home' })).toBeVisible()

  // Purposefully break signatures
  await page.evaluate(() => {
    localStorage.setItem('publicKeyID', 'fake!')
  })
  await page.goto('/')

  // Need this to make sure getByText polling doesn't miss the auth page
  await page.waitForURL('**/authenticate**')

  await expect(page.getByText('Authenticate UI Access')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Home' })).toBeVisible()
})
