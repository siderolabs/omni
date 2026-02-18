// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import {
  TalosDiscoveredVolumeType,
  TalosDiskType,
  TalosMountStatusType,
  TalosRuntimeNamespace,
} from '@/api/resources'

import NodeDisks, {
  type TalosDiscoveredVolumeSpec,
  type TalosDiskSpec,
  type TalosMountStatusSpec,
} from './NodeDisks.vue'

const meta: Meta<typeof NodeDisks> = {
  component: NodeDisks,
}

export default meta
type Story = StoryObj<typeof meta>

export const WithData: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<TalosDiskSpec>({
          expectedOptions: {
            namespace: TalosRuntimeNamespace,
            type: TalosDiskType,
          },
          initialResources: [
            {
              metadata: {
                id: 'vda',
                namespace: TalosRuntimeNamespace,
                type: TalosDiskType,
              },
              spec: {
                dev_path: '/dev/vda',
                pretty_size: '20 GB',
                size: 21474836480,
                transport: 'virtio',
              },
            },
          ],
        }).handler,

        createWatchStreamHandler<TalosDiscoveredVolumeSpec>({
          expectedOptions: {
            namespace: TalosRuntimeNamespace,
            type: TalosDiscoveredVolumeType,
          },
          initialResources: [
            {
              metadata: {
                id: 'vda',
                namespace: TalosRuntimeNamespace,
                type: TalosDiscoveredVolumeType,
              },
              spec: {
                dev_path: '/dev/vda',
                pretty_size: '20 GB',
                size: 21474836480,
                type: 'disk',
              },
            },
            {
              metadata: {
                id: 'vda1',
                namespace: TalosRuntimeNamespace,
                type: TalosDiscoveredVolumeType,
              },
              spec: {
                dev_path: '/dev/vda1',
                name: 'vfat',
                parent: 'vda',
                partition_index: 1,
                partition_label: 'EFI',
                pretty_size: '105 MB',
                size: 105906176,
                type: 'partition',
              },
            },
            {
              metadata: {
                id: 'vda2',
                namespace: TalosRuntimeNamespace,
                type: TalosDiscoveredVolumeType,
              },
              spec: {
                dev_path: '/dev/vda2',
                name: 'xfs',
                parent: 'vda',
                partition_index: 2,
                partition_label: 'BOOT',
                pretty_size: '1.0 GB',
                size: 1073741824,
                type: 'partition',
              },
            },
            {
              metadata: {
                id: 'vda3',
                namespace: TalosRuntimeNamespace,
                type: TalosDiscoveredVolumeType,
              },
              spec: {
                dev_path: '/dev/vda3',
                name: 'xfs',
                parent: 'vda',
                partition_index: 3,
                partition_label: 'STATE',
                pretty_size: '104 MB',
                size: 104857600,
                type: 'partition',
              },
            },
            {
              metadata: {
                id: 'vda4',
                namespace: TalosRuntimeNamespace,
                type: TalosDiscoveredVolumeType,
              },
              spec: {
                dev_path: '/dev/vda4',
                name: 'xfs',
                parent: 'vda',
                partition_index: 4,
                partition_label: 'EPHEMERAL',
                pretty_size: '4.1 GB',
                size: 4131389440,
                uuid: 'faa1a56a-4934-4317-b6d1-272712353c5c',
                type: 'partition',
              },
            },
          ],
        }).handler,

        createWatchStreamHandler<TalosMountStatusSpec>({
          expectedOptions: {
            namespace: TalosRuntimeNamespace,
            type: TalosMountStatusType,
          },
          initialResources: [
            {
              metadata: {
                id: '/dev/vda4',
                namespace: TalosRuntimeNamespace,
                type: TalosMountStatusType,
              },
              spec: {
                encrypted: true,
                filesystemType: 'xfs',
                source: '/dev/vda4',
                target: '/var',
              },
            },
          ],
        }).handler,
      ],
    },
  },
}

export const NoData: Story = {
  parameters: {
    msw: {
      handlers: [createWatchStreamHandler().handler],
    },
  },
}
