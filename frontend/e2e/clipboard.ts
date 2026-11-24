// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Page } from '@playwright/test'

/**
 * Reads the clipboard, meant to be called from `expect.poll`.
 *
 * Resolves to `null` instead of throwing when the read is refused, so the poll retries rather
 * than failing outright: Firefox can reject a read that lands while the app is still copying
 * (e.g. while it awaits a request before writing).
 */
export const readClipboard = (page: Page) =>
  page.evaluate(() => navigator.clipboard.readText()).catch(() => null)

/**
 * Reads space-separated kernel parameters from the clipboard into a `key -> value` map, meant
 * to be called from `expect.poll`. Resolves to `null` when the clipboard can't be read.
 */
export const readKernelArgs = async (page: Page) => {
  const value = await readClipboard(page)
  if (value === null) return null

  return Object.fromEntries(
    value.split(' ').map((arg) => {
      const [key, ...rest] = arg.split('=')
      return [key, rest.join('=')]
    }),
  )
}
