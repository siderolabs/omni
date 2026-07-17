// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import dns from 'node:dns/promises'
import path from 'node:path'

import { defineConfig, devices } from '@playwright/test'
import dotenv from 'dotenv'

/**
 * Read environment variables from file.
 * https://github.com/motdotla/dotenv
 */
dotenv.config({ quiet: true })

/**
 * Chromium resolves any *.localhost hostname to loopback without consulting the OS resolver,
 * so when the tests run in a container that reaches Omni through an --add-host mapping of
 * such a hostname, the mapping never takes effect in the browser. Repeat it through
 * Chromium's own resolver rules, using the address the OS resolver returns. The rule
 * wildcard spans multiple labels, so *.omni.localhost also covers workload proxy hostnames
 * like nginx-my-instance.proxy-us.omni.localhost.
 */
async function chromiumArgs(): Promise<string[]> {
  const host = process.env.BASE_URL ? new URL(process.env.BASE_URL).hostname : undefined
  if (!host?.endsWith('.localhost')) return []

  let address: string
  try {
    ;({ address } = await dns.lookup(host))
  } catch {
    return []
  }

  if (['127.0.0.1', '::1'].includes(address)) return []

  const parentDomain = host.split('.').slice(1).join('.')
  const target = address.includes(':') ? `[${address}]` : address

  return [`--host-resolver-rules=MAP *.${parentDomain} ${target}`]
}

/**
 * See https://playwright.dev/docs/test-configuration.
 */
export default defineConfig({
  testDir: path.join('.', 'e2e'),
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 2 : undefined,
  reporter: process.env.CI ? [['html'], ['list'], ['github']] : [['html'], ['list']],
  use: {
    baseURL: process.env.BASE_URL,
    launchOptions: { args: await chromiumArgs() },
    permissions: ['clipboard-read', 'clipboard-write'],
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
    ignoreHTTPSErrors: true,
    video: {
      mode: process.env.CI ? 'retain-on-failure' : 'on',
      size: { width: 1280, height: 720 },
    },
  },

  projects: [
    {
      name: 'eula',
      testMatch: 'eula/**/*.spec.ts',
    },
    {
      name: 'talemu-setup',
      testMatch: 'talemu/talemu.setup.ts',
      // The setup uses the auth fixture, which assumes the EULA is already
      // accepted. Without this dependency talemu-setup races the eula project
      // and intermittently gets stuck on the EULA gate ("setting up auth" timeout).
      dependencies: ['eula'],
    },
    {
      name: 'auth0',
      use: {
        ...devices['Desktop Chrome'],
      },
      testMatch: 'auth0/**/*.spec.ts',
      dependencies: ['eula'],
    },
    {
      name: 'talemu',
      use: {
        ...devices['Desktop Chrome'],
      },
      testMatch: 'talemu/**/*.spec.ts',
      dependencies: ['eula', 'talemu-setup'],
    },
    {
      name: 'qemu',
      use: {
        ...devices['Desktop Chrome'],
      },
      testMatch: 'qemu/**/*.spec.ts',
      dependencies: ['eula'],
    },
  ],
})
