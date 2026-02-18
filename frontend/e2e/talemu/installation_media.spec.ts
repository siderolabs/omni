// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { stat } from 'node:fs/promises'

import { faker } from '@faker-js/faker'
import { milliseconds } from 'date-fns'
import { load } from 'js-yaml'

import { expect, test } from '../auth_fixtures'

test.describe.configure({ mode: 'parallel' })

test('Download installation media', async ({ page }, testInfo) => {
  test.slow()

  await test.step('Go to wizard', async () => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Download Installation Media' }).click()

    const firstStepHeading = page.getByRole('heading', { name: 'Create New Media' })

    const isVisible = await firstStepHeading
      .waitFor({ timeout: milliseconds({ seconds: 2 }) })
      .then(() => true)
      .catch(() => false)

    // Only click "Create New" if we don't auto redirect (redirects directly to wizard if no configs exist)
    // eslint-disable-next-line playwright/no-conditional-in-test
    if (!isVisible) {
      await page.getByRole('link', { name: 'Create New' }).click()
    }

    await expect(firstStepHeading).toBeVisible()
  })

  await test.step('Entry step', async () => {
    await page.getByRole('radio', { name: 'Bare-metal Machine' }).click()
    await expect(page.getByRole('radio', { name: 'Bare-metal Machine' })).toBeChecked()

    await page.getByRole('link', { name: 'Next' }).click()
  })

  await test.step('Talos version step', async () => {
    await page.getByRole('combobox', { name: 'Choose Talos Linux Version' }).click()
    await page.getByRole('option', { name: '1.12.0' }).click()
    await expect(page.getByRole('combobox', { name: 'Choose Talos Linux Version' })).toHaveText(
      '1.12.0',
    )

    await page.getByRole('combobox', { name: 'Join Token' }).click()
    await page.getByRole('option', { name: 'initial token' }).click()
    await expect(page.getByRole('combobox', { name: 'Join Token' })).toHaveText('initial token')

    await page.getByText('Tunnel Omni management').click()
    await expect(page.getByRole('checkbox', { name: 'Tunnel Omni management' })).toBeChecked()

    await page.getByRole('button', { name: 'new label' }).click()
    await page.getByRole('textbox').first().fill('foo:bar')
    await page.getByRole('textbox').first().press('Enter')
    await expect(page.getByRole('button', { name: 'foo:bar' })).toBeVisible()

    await page.getByRole('link', { name: 'Next' }).click()
  })

  await test.step('Architecture step', async () => {
    await page.getByRole('radio', { name: 'arm64' }).click()
    await expect(page.getByRole('radio', { name: 'arm64' })).toBeChecked()

    await page.getByRole('checkbox', { name: 'SecureBoot' }).click()
    await expect(page.getByRole('checkbox', { name: 'SecureBoot' })).toBeChecked()

    await page.getByRole('link', { name: 'Next' }).click()
  })

  await test.step('System extensions step', async () => {
    await page.getByPlaceholder('Search').fill('hello')
    await page.getByText('siderolabs/hello-world-service').click()
    await expect(
      page.getByRole('checkbox', { name: 'siderolabs/hello-world-service' }),
    ).toBeChecked()

    await page.getByRole('link', { name: 'Next' }).click()
  })

  await test.step('Extra args step', async () => {
    await page.getByRole('radio', { name: 'Auto' }).click()
    await page.locator('.flex.max-h-full').first().click()

    await page
      .getByRole('textbox', { name: 'Extra kernel command line' })
      .fill(`-console console=tty0`)

    await page.getByRole('link', { name: 'Next' }).click()
  })

  const savedPresetName = `e2e-media-${faker.string.alphanumeric(8)}`

  let schematicId: string

  await test.step('Confirmation step', async () => {
    await page.getByRole('button', { name: 'Copy schematic ID' }).click()

    schematicId = await page.evaluate(() => navigator.clipboard.readText())
    expect(schematicId, 'Expect schematic ID to be valid').toMatch(/[a-zA-Z0-9]{64}/)

    await page.getByRole('button', { name: 'Copy schematic YAML' }).click()

    const schematicYml = await page.evaluate(() => navigator.clipboard.readText())
    const parsedSchematicYml = load(schematicYml)
    await testInfo.attach('schematic.yaml', {
      body: schematicYml,
      contentType: 'application/yaml',
    })

    expect(parsedSchematicYml, 'Expect YAML to match expected shape').toEqual({
      customization: {
        extraKernelArgs: [
          expect.stringContaining('siderolink.api=grpc://'),
          expect.stringContaining('talos.events.sink='),
          expect.stringContaining('talos.logging.kernel='),
          '-console',
          'console=tty0',
        ],
        meta: [{ key: 12, value: expect.any(String) }],
        systemExtensions: {
          officialExtensions: ['siderolabs/hello-world-service'],
        },
      },
    })

    await expect(
      page.getByText(
        `https://factory.talos.dev/image/${schematicId}/1.12.0/metal-arm64-secureboot.iso`,
      ),
    ).toBeVisible()

    await expect(
      page.getByText(
        `https://factory.talos.dev/image/${schematicId}/1.12.0/metal-arm64-secureboot.raw.zst`,
      ),
    ).toBeVisible()

    await expect(
      page.getByText(`https://pxe.factory.talos.dev/${schematicId}/1.12.0/metal-arm64-secureboot`),
    ).toBeVisible()

    await page.getByRole('button', { name: 'Save' }).click()
    await page.getByRole('textbox', { name: 'Name:' }).fill(savedPresetName)
    await page.getByRole('textbox', { name: 'Name:' }).click()
    await page.getByLabel('Save preset').getByRole('button', { name: 'Save' }).click()
    await page.getByRole('link', { name: 'Finished' }).click()
  })

  const presetRow = page.getByRole('row', { name: savedPresetName })

  await test.step('Download the image', async () => {
    await presetRow.getByLabel('download').click()

    const isoRow = page.getByRole('row', { name: 'SecureBoot ISO' })
    await isoRow.getByLabel('copy link').click()

    const isoLink = await page.evaluate(() => navigator.clipboard.readText())
    expect(isoLink).toBe(
      `https://factory.talos.dev/image/${schematicId}/1.12.0/metal-arm64-secureboot.iso`,
    )

    await isoRow.getByLabel('download').click()

    const [download] = await Promise.all([
      page.waitForEvent('download'),
      expect(page.getByText('Generating Image')).toBeVisible(),
    ])

    const filePath = testInfo.outputPath(download.suggestedFilename())
    await download.saveAs(filePath)

    const { size } = await stat(filePath)

    expect(size, 'Expect ISO to be at least 50MB').toBeGreaterThan(50 * 1024 * 1024)

    await page.getByRole('button', { name: 'Close', exact: true }).click()
  })

  await test.step('Delete the image', async () => {
    await presetRow.getByLabel('delete').click()

    await expect(
      page.getByText(`Are you sure you want to delete preset "${savedPresetName}"?`),
    ).toBeVisible()

    await page.getByRole('button', { name: 'Confirm' }).click()

    await expect(page.getByText(`Deleted preset ${savedPresetName}`)).toBeVisible()
    await expect(presetRow).toBeHidden()
  })
})
