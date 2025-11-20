// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from './auth_fixtures'

/**
 * The nonce CSP header & tag are required by userpilot scripts
 */
test('includes unique nonce in the CSP', async ({ page }) => {
  async function verifyCsp() {
    const a = await page.goto('/')

    const cspHeader = await a?.headerValue('content-security-policy')
    const nonceRegex = /'nonce-(.*?)'/
    const nonce = nonceRegex.exec(cspHeader ?? '')?.[1]

    expect.soft(cspHeader, 'nonce header is set').toMatch(nonceRegex)
    expect.soft(nonce, 'nonce header has a value').toBeTruthy()
    await expect
      .soft(page.locator("meta[name='csp-nonce']"), 'nonce meta tag has matching value')
      .toHaveAttribute('content', nonce!)

    return nonce
  }

  const firstCsp = await verifyCsp()
  const secondCsp = await verifyCsp()

  expect(firstCsp, 'nonces are unique per request').not.toBe(secondCsp)
})
