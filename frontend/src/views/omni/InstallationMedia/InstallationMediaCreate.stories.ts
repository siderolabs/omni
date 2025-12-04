// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import InstallationMediaCreate from './InstallationMediaCreate.vue'
import * as CloudProviderStories from './Steps/CloudProvider.stories'
import * as ExternalArgsStories from './Steps/ExternalArgs.stories'
import * as MachineArchStories from './Steps/MachineArch.stories'
import * as SBCTypeStories from './Steps/SBCType.stories'
import * as SystemExtensionsStories from './Steps/SystemExtensions.stories'
import * as TalosVersionStories from './Steps/TalosVersion.stories'

const meta: Meta<typeof InstallationMediaCreate> = {
  component: InstallationMediaCreate,
  parameters: {
    layout: 'fullscreen',
  },
  args: {
    class: 'h-screen',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        ...CloudProviderStories.Default.parameters.msw.handlers,
        ...ExternalArgsStories.Default.parameters.msw.handlers,
        ...MachineArchStories.Default.parameters.msw.handlers,
        ...SBCTypeStories.Default.parameters.msw.handlers,
        ...SystemExtensionsStories.Default.parameters.msw.handlers,
        ...TalosVersionStories.Default.parameters.msw.handlers,
      ],
    },
  },
}
