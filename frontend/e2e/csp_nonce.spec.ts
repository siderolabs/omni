// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from './auth_fixtures'

/**
 * The nonce CSP header is required by userpilot scripts
 */
test('Has correct csp-nonce meta tag', async ({ page }) => {
  const a = await page.goto('/')

  const cspHeader = a?.headers()['content-security-policy']
  const nonceRegex = /'nonce-(.*?)'/
  const nonce = nonceRegex.exec(cspHeader ?? '')?.[1]

  expect.soft(cspHeader, 'nonce header is set').toMatch(nonceRegex)
  expect.soft(nonce, 'nonce header has a value').toBeTruthy()
  await expect
    .soft(page.locator("meta[name='csp-nonce']"), 'nonce meta tag has matching value')
    .toHaveAttribute('content', nonce!)
})
