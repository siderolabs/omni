// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { test as teardown } from '../auth_fixtures'
import { DEFAULT_MACHINE_CLASS } from '../constants'

teardown('remove default machine class', async ({ page }) => {
  await teardown.step('Visit machine classes page', async () => {
    await page.goto('/')
    await page.getByRole('button', { name: 'Machine Management' }).click()
    await page.getByRole('link', { name: 'Classes' }).click()
  })

  const classRow = page.getByRole('row').filter({ hasText: DEFAULT_MACHINE_CLASS })

  await teardown.step('Delete machine class', async () => {
    await classRow.getByRole('button', { name: 'delete' }).click()
    await page.getByRole('button', { name: 'Destroy' }).click()
  })

  await teardown.step('Assert class deletion', async () => {
    await page.getByText('Please confirm the action').waitFor({ state: 'detached' })
    await classRow.waitFor({ state: 'detached' })
  })
})
