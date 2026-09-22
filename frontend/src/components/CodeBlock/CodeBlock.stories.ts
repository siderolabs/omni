// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import CodeBlock from './CodeBlock.vue'

const meta: Meta<typeof CodeBlock> = {
  component: CodeBlock,
  args: {
    code: 'brew install siderolabs/tap/sidero-toolsbrew install siderolabs/tap/sidero-toolsbrew install siderolabs/tap/sidero-toolsbrew install siderolabs/tap/sidero-toolsbrew install siderolabs/tap/sidero-tools',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const Yaml: Story = {
  args: {
    lang: 'yaml',
    code: [
      '# Machine configuration patch',
      'machine:',
      '  network:',
      '    hostname: talos-node-1',
      '    interfaces:',
      '      - interface: eth0',
      '        dhcp: true',
      '  install:',
      '    wipe: false',
    ].join('\n'),
  },
}

const json = JSON.stringify(
  { id: 'talos-node-1', connected: true, addresses: ['10.5.0.2'] },
  null,
  2,
)

export const Json: Story = {
  args: {
    lang: 'json',
    code: json,
  },
}

/** Search matches are tinted on top of the syntax colours. */
export const WithSearch: Story = {
  args: {
    lang: 'json',
    code: json,
    search: 'node',
  },
}
