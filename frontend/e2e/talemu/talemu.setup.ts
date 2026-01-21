// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { test as setup } from '../auth_fixtures'
import { DEFAULT_MACHINE_CLASS } from '../constants'

setup('setup default machine class', async ({ page }) => {
  await setup.step('Visit machine classes page', async () => {
    await page.goto('/')
    await page.getByRole('button', { name: 'Machine Management' }).click()
    await page.getByRole('link', { name: 'Classes' }).click()
  })

  await setup.step('Create machine class', async () => {
    await page.getByRole('link', { name: 'Create Machine Class' }).click()

    await page.getByRole('textbox', { name: 'Machine Class Name' }).fill(DEFAULT_MACHINE_CLASS)
    await page.getByRole('button', { name: 'Auto Provision' }).click()
    await page.getByText('id: Talemu').click()

    await page.getByRole('button', { name: 'Create Machine Class' }).click()
  })

  await setup.step('Assert class creation', async () => {
    await page.getByRole('heading', { name: 'Machine Classes' }).waitFor()
    await page.getByText(DEFAULT_MACHINE_CLASS).waitFor()
  })
})
