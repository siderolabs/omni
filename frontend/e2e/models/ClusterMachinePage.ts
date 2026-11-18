// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Page } from '@playwright/test'

/** The tabs that group several pages, and the pages in each one's second row. */
const sections = {
  Configuration: ['Current', 'Patches', 'Pending Updates', 'Change History'],
  System: ['Disks', 'Devices', 'Extensions', 'Kernel Args', 'KubeSpan'],
} as const

type Section = keyof typeof sections

/** A machine's page within a cluster, from its tabs to the pages they group. See MachinePage for one outside a cluster. */
export class ClusterMachinePage {
  constructor(readonly page: Page) {}

  /** Opens a tab that is a single page. */
  async open(tab: 'Overview' | 'Monitor' | 'Logs'): Promise<void>
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
