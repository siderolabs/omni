// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import path from 'node:path'

import { defineConfig, devices } from '@playwright/test'
import dotenv from 'dotenv'

/**
 * Read environment variables from file.
 * https://github.com/motdotla/dotenv
 */
dotenv.config({ quiet: true })

const permissions = ['clipboard-read', 'clipboard-write']

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
    permissions,
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
      name: 'saml',
      use: {
        ...devices['Desktop Chrome'],
      },
      testMatch: 'saml/**/*.spec.ts',
      dependencies: ['eula'],
    },
    {
      name: 'talemu-chrome',
      use: {
        ...devices['Desktop Chrome'],
      },
      testMatch: 'talemu/**/*.spec.ts',
      dependencies: ['eula', 'talemu-setup'],
    },
    {
      name: 'talemu-firefox',
      use: {
        ...devices['Desktop Firefox'],
        // Firefox does not support clipboard permissions
        permissions: permissions.filter((p) => !p.startsWith('clipboard')),
        launchOptions: {
          firefoxUserPrefs: {
            // Firefox specific flag to always allow clipboard access
            'dom.events.testing.asyncClipboard': true,
          },
        },
      },
      testMatch: 'talemu/**/*.spec.ts',
      dependencies: ['eula', 'talemu-setup'],
    },
    {
      name: 'talemu',
      testMatch: /(?!)/,
      dependencies: ['talemu-chrome', 'talemu-firefox'],
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
