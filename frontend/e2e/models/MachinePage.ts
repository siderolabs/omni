// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Page } from '@playwright/test'

/** The tabs that group several pages, and the pages in each one's second row. */
const sections = {
  System: ['Disks', 'Devices', 'Extensions', 'Kernel Args'],
} as const

type Section = keyof typeof sections

/** A machine's page outside of any cluster, from the Machines list. See ClusterMachinePage for one in a cluster. */
export class MachinePage {
  constructor(readonly page: Page) {}

  /** Opens a tab that is a single page. Configuration opens the machine's patches. */
  async open(tab: 'Logs' | 'Configuration'): Promise<void>
  /** Opens a tab that groups several pages, then one of the pages in its second row. */
  async open<T extends Section>(tab: T, subPage: (typeof sections)[T][number]): Promise<void>
  async open(tab: string, subPage?: string) {
    await this.page
      .getByRole('navigation', { name: 'Machine' })
      .getByRole('link', { name: tab, exact: true })
      .click()

    if (subPage) {
      // The second row is a nav named after the tab it belongs to
      await this.page
        .getByRole('navigation', { name: tab })
        .getByRole('link', { name: subPage, exact: true })
        .click()
    }
  }
}
