// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { vueRouter } from 'storybook-vue3-router'

import { handlers as machineArchHandlers } from '@/pages/(authenticated)/machines/installation-media/create/arch.mocks'
import MachineArch from '@/pages/(authenticated)/machines/installation-media/create/arch.vue'
import { handlers as cloudProviderHandlers } from '@/pages/(authenticated)/machines/installation-media/create/cloud-provider.mocks'
import CloudProvider from '@/pages/(authenticated)/machines/installation-media/create/cloud-provider.vue'
import { handlers as confirmationHandlers } from '@/pages/(authenticated)/machines/installation-media/create/confirmation.mocks'
import Confirmation from '@/pages/(authenticated)/machines/installation-media/create/confirmation.vue'
import { handlers as extraArgsHandlers } from '@/pages/(authenticated)/machines/installation-media/create/extra-args.mocks'
import ExtraArgs from '@/pages/(authenticated)/machines/installation-media/create/extra-args.vue'
import Entry from '@/pages/(authenticated)/machines/installation-media/create/index.vue'
import { handlers as sbcTypeHandlers } from '@/pages/(authenticated)/machines/installation-media/create/sbc-type.mocks'
import SBCType from '@/pages/(authenticated)/machines/installation-media/create/sbc-type.vue'
import { handlers as systemExtensionHandlers } from '@/pages/(authenticated)/machines/installation-media/create/system-extensions.mocks'
import SystemExtensions from '@/pages/(authenticated)/machines/installation-media/create/system-extensions.vue'
import { handlers as talosVersionHandlers } from '@/pages/(authenticated)/machines/installation-media/create/talos-version.mocks'
import TalosVersion from '@/pages/(authenticated)/machines/installation-media/create/talos-version.vue'
import { handlers as savePresetModalHandlers } from '@/views/InstallationMedia/SavePresetModal.mocks'

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

  beforeEach({ msw }) {
    msw.use(
      ...savePresetModalHandlers,
      ...cloudProviderHandlers,
      ...confirmationHandlers,
      ...extraArgsHandlers,
      ...machineArchHandlers,
      ...sbcTypeHandlers,
      ...systemExtensionHandlers,
      ...talosVersionHandlers,
    )
  },
}
