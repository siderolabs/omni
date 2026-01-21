// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { vueRouter } from 'storybook-vue3-router'

import * as SavePresetModalStories from '@/views/omni/InstallationMedia/SavePresetModal.stories'
import * as CloudProviderStories from '@/views/omni/InstallationMedia/Steps/CloudProvider.stories'
import CloudProvider from '@/views/omni/InstallationMedia/Steps/CloudProvider.vue'
import * as ConfirmationStories from '@/views/omni/InstallationMedia/Steps/Confirmation.stories'
import Confirmation from '@/views/omni/InstallationMedia/Steps/Confirmation.vue'
import Entry from '@/views/omni/InstallationMedia/Steps/Entry.vue'
import * as ExtraArgsStories from '@/views/omni/InstallationMedia/Steps/ExtraArgs.stories'
import ExtraArgs from '@/views/omni/InstallationMedia/Steps/ExtraArgs.vue'
import * as MachineArchStories from '@/views/omni/InstallationMedia/Steps/MachineArch.stories'
import MachineArch from '@/views/omni/InstallationMedia/Steps/MachineArch.vue'
import * as SBCTypeStories from '@/views/omni/InstallationMedia/Steps/SBCType.stories'
import SBCType from '@/views/omni/InstallationMedia/Steps/SBCType.vue'
import * as SystemExtensionsStories from '@/views/omni/InstallationMedia/Steps/SystemExtensions.stories'
import SystemExtensions from '@/views/omni/InstallationMedia/Steps/SystemExtensions.vue'
import * as TalosVersionStories from '@/views/omni/InstallationMedia/Steps/TalosVersion.stories'
import TalosVersion from '@/views/omni/InstallationMedia/Steps/TalosVersion.vue'

import InstallationMediaCreate from './InstallationMediaCreate.vue'

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
  decorators: [
    vueRouter(
      [
        {
          path: '/entry',
          name: 'InstallationMediaCreateEntry',
          component: Entry,
        },
        {
          path: '/talos-version',
          name: 'InstallationMediaCreateTalosVersion',
          component: TalosVersion,
        },
        {
          path: '/cloud-provider',
          name: 'InstallationMediaCreateCloudProvider',
          component: CloudProvider,
        },
        {
          path: '/sbc-type',
          name: 'InstallationMediaCreateSBCType',
          component: SBCType,
        },
        {
          path: '/arch',
          name: 'InstallationMediaCreateMachineArch',
          component: MachineArch,
        },
        {
          path: '/system-extensions',
          name: 'InstallationMediaCreateSystemExtensions',
          component: SystemExtensions,
        },
        {
          path: '/extra-args',
          name: 'InstallationMediaCreateExtraArgs',
          component: ExtraArgs,
        },
        {
          path: '/confirmation',
          name: 'InstallationMediaCreateConfirmation',
          component: Confirmation,
        },
      ],
      { initialRoute: '/entry' },
    ),
  ],
  parameters: {
    msw: {
      handlers: [
        ...SavePresetModalStories.Default.parameters.msw.handlers,
        ...CloudProviderStories.Default.parameters.msw.handlers,
        ...ConfirmationStories.Default.parameters.msw.handlers,
        ...ExtraArgsStories.Default.parameters.msw.handlers,
        ...MachineArchStories.Default.parameters.msw.handlers,
        ...SBCTypeStories.Default.parameters.msw.handlers,
        ...SystemExtensionsStories.Default.parameters.msw.handlers,
        ...TalosVersionStories.Default.parameters.msw.handlers,
      ],
    },
  },
}
