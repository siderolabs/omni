// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createResourceListHandler, createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import type { ClusterSpec } from '@/api/omni/specs/omni.pb'
import type { SBCConfigSpec } from '@/api/omni/specs/virtual.pb'
import { ClusterType, DefaultNamespace, SBCConfigType, VirtualNamespace } from '@/api/resources'
import {
  commonHandlers,
  machineStatuses,
  TalosApiCertificateType,
  TalosSecretsNamespace,
} from '@/views/Internals/Internals.mocks'

import ResourceEntryList from './ResourceEntryList.vue'

const meta: Meta<typeof ResourceEntryList> = {
  component: ResourceEntryList,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [
    () => ({
      template: '<div class="h-screen"><story /></div>',
    }),
  ],
}

export default meta
type Story = StoryObj<typeof meta>

export const Omni: Story = {
  args: {
    target: { runtime: 'omni' },
    type: ClusterType,
  },
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler<ClusterSpec>({
        expectedOptions: {
          namespace: DefaultNamespace,
          type: ClusterType,
        },
        initialResources: ['talos-default', 'production', 'staging'].map((id) => ({
          metadata: {
            namespace: DefaultNamespace,
            type: ClusterType,
            id,
            version: String(faker.number.int({ min: 1, max: 20 })),
            phase: 'running',
            created: faker.date.recent({ days: 7 }).toISOString(),
            updated: faker.date.recent().toISOString(),
          },
          spec: {
            kubernetes_version: '1.34.1',
            talos_version: '1.11.3',
          },
        })),
      }).handler,
      ...commonHandlers,
    )
  },
}

export const TalosSensitive: Story = {
  args: {
    target: { runtime: 'talos', machine: machineStatuses[0].metadata.id! },
    type: TalosApiCertificateType,
  },
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler({
        expectedOptions: {
          namespace: TalosSecretsNamespace,
          type: TalosApiCertificateType,
        },
        initialResources: [
          {
            metadata: {
              namespace: TalosSecretsNamespace,
              type: TalosApiCertificateType,
              id: 'api',
              version: '3',
              owner: 'secrets.APIController',
              phase: 'running',
              created: faker.date.recent().toISOString(),
              updated: faker.date.recent().toISOString(),
            },
            spec: {
              acceptedCAs: [{ crt: 'LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t' }],
              client: {
                crt: 'LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t',
                key: 'LS0tLS1CRUdJTiBFRDI1NTE5IFBSSVZBVEUgS0VZLS0tLS0K',
              },
              server: {
                crt: 'LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t',
                key: 'LS0tLS1CRUdJTiBFRDI1NTE5IFBSSVZBVEUgS0VZLS0tLS0K',
              },
              skipVerifyingClientCert: false,
            },
          },
        ],
      }).handler,
      ...commonHandlers,
    )
  },
}

export const Virtual: Story = {
  args: {
    target: { runtime: 'omni' },
    type: SBCConfigType,
  },
  beforeEach({ msw }) {
    msw.use(
      createResourceListHandler<SBCConfigSpec>({
        expectedOptions: {
          namespace: VirtualNamespace,
          type: SBCConfigType,
        },
        resources: ['rpi_generic', 'rock5b', 'jetson_nano'].map((id) => ({
          metadata: {
            namespace: VirtualNamespace,
            type: SBCConfigType,
            id,
            version: '1',
            phase: 'running',
            // Virtual resources are generated per request, so this is hidden
            updated: faker.date.recent().toISOString(),
          },
          spec: {
            label: id,
            overlay_name: id,
            overlay_image: `siderolabs/sbc-${id}`,
          },
        })),
      }),
      ...commonHandlers,
    )
  },
}

export const NoResources: Story = {
  args: {
    target: { runtime: 'omni' },
    type: ClusterType,
  },
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler({
        expectedOptions: {
          namespace: DefaultNamespace,
          type: ClusterType,
        },
      }).handler,
      ...commonHandlers,
    )
  },
}

export const UnknownType: Story = {
  args: {
    target: { runtime: 'omni' },
    type: 'Nonexistents.omni.sidero.dev',
  },
  beforeEach({ msw }) {
    msw.use(...commonHandlers)
  },
}
