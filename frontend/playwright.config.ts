// Copyright (c) 2025 Sidero Labs, Inc.
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
      name: 'talemu-setup',
      testMatch: 'talemu/talemu.setup.ts',
      teardown: 'talemu-teardown',
    },
    {
      name: 'talemu-teardown',
      testMatch: 'talemu/talemu.teardown.ts',
    },
    {
      name: 'talemu',
      use: {
        ...devices['Desktop Chrome'],
      },
      testMatch: 'talemu/**/*.spec.ts',
      dependencies: ['talemu-setup'],
    },
    {
      name: 'qemu',
      use: {
        ...devices['Desktop Chrome'],
      },
      testMatch: 'qemu/**/*.spec.ts',
    },
  ],
})
