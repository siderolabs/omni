// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { vueRouter } from 'storybook-vue3-router'

import * as MachineArchStories from '@/pages/(authenticated)/machines/installation-media/create/arch.stories'
import MachineArch from '@/pages/(authenticated)/machines/installation-media/create/arch.vue'
import * as CloudProviderStories from '@/pages/(authenticated)/machines/installation-media/create/cloud-provider.stories'
import CloudProvider from '@/pages/(authenticated)/machines/installation-media/create/cloud-provider.vue'
import * as ConfirmationStories from '@/pages/(authenticated)/machines/installation-media/create/confirmation.stories'
import Confirmation from '@/pages/(authenticated)/machines/installation-media/create/confirmation.vue'
import * as ExtraArgsStories from '@/pages/(authenticated)/machines/installation-media/create/extra-args.stories'
import ExtraArgs from '@/pages/(authenticated)/machines/installation-media/create/extra-args.vue'
import Entry from '@/pages/(authenticated)/machines/installation-media/create/index.vue'
import * as SBCTypeStories from '@/pages/(authenticated)/machines/installation-media/create/sbc-type.stories'
import SBCType from '@/pages/(authenticated)/machines/installation-media/create/sbc-type.vue'
import * as SystemExtensionsStories from '@/pages/(authenticated)/machines/installation-media/create/system-extensions.stories'
import SystemExtensions from '@/pages/(authenticated)/machines/installation-media/create/system-extensions.vue'
import * as TalosVersionStories from '@/pages/(authenticated)/machines/installation-media/create/talos-version.stories'
import TalosVersion from '@/pages/(authenticated)/machines/installation-media/create/talos-version.vue'
import * as SavePresetModalStories from '@/views/omni/InstallationMedia/SavePresetModal.stories'

import InstallationMediaCreate from './create.vue'

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
