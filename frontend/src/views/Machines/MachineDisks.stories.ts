// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import {
  TalosDiscoveredVolumeType,
  TalosDiskType,
  TalosRuntimeNamespace,
  TalosVolumeStatusType,
} from '@/api/resources'
import {
  type DiscoveredVolumeSpec,
  type DiskSpec,
  type VolumeStatusSpec,
} from '@/api/talos/block.pb'

import NodeDisks from './MachineDisks.vue'

const meta: Meta<typeof NodeDisks> = {
  component: NodeDisks,
}

export default meta
type Story = StoryObj<typeof meta>

const handlers = [
  createWatchStreamHandler<DiskSpec>({
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
          size: 21474836480,
          transport: 'virtio',
        },
      },
    ],
  }).handler,
  createWatchStreamHandler<DiscoveredVolumeSpec>({
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
          size: 4131389440,
          uuid: faker.string.uuid(),
          type: 'partition',
        },
      },
      {
        metadata: {
          id: 'vda5',
          namespace: TalosRuntimeNamespace,
          type: TalosDiscoveredVolumeType,
        },
        spec: {
          dev_path: '/dev/vda4',
          name: 'xfs',
          parent: 'vda',
          partition_index: 4,
          partition_label: 'USER',
          size: 3267896320,
          uuid: faker.string.uuid(),
          type: 'partition',
        },
      },
    ],
  }).handler,
  createWatchStreamHandler<VolumeStatusSpec>({
    expectedOptions: {
      namespace: TalosRuntimeNamespace,
      type: TalosVolumeStatusType,
    },
    initialResources: [
      {
        metadata: {
          id: 'EPHEMERAL',
          namespace: TalosRuntimeNamespace,
          type: TalosVolumeStatusType,
        },
        spec: {
          filesystem: 'xfs',
          location: '/dev/vda4',
          mountLocation: '/dev/vda4',
          mountSpec: {
            fileMode: 493,
            projectQuotaSupport: true,
            selinuxLabel: 'system_u:object_r:ephemeral_t:s0',
            targetPath: '/var',
          },
          parentLocation: '/dev/vda',
          partitionIndex: 4,
          phase: 'ready',
          prettySize: '4.1 GB',
          size: 4131389440,
          type: 'partition',
        },
      },
      {
        metadata: {
          id: 'u-mydata2',
          namespace: TalosRuntimeNamespace,
          type: TalosVolumeStatusType,
        },
        spec: {
          configuredEncryptionKeys: ['static'],
          encryptionProvider: 'luks2',
          encryptionSlot: 0,
          filesystem: 'xfs',
          location: '/dev/vda3',
          mountLocation: '/dev/dm-0',
          mountSpec: {
            fileMode: 493,
            projectQuotaSupport: false,
            selinuxLabel: 'system_u:object_r:ephemeral_t:s0',
            targetPath: 'mydata2',
          },
          parentLocation: '/dev/vda',
          partitionIndex: 3,
          phase: 'ready',
          prettySize: '104 MB',
          size: 104857600,
          type: 'partition',
        },
      },
    ],
  }).handler,
]

export const Default: Story = {
  beforeEach({ msw }) {
    msw.use(...handlers)
  },
}

export const WithEmptyVolume: Story = {
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler<DiscoveredVolumeSpec>({
        expectedOptions: {
          namespace: TalosRuntimeNamespace,
          type: TalosDiscoveredVolumeType,
        },
        initialResources: [],
      }).handler,
      ...handlers,
    )
  },
}

export const WithCdrom: Story = {
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler<DiskSpec>({
        expectedOptions: {
          namespace: TalosRuntimeNamespace,
          type: TalosDiskType,
        },
        initialResources: [
          {
            metadata: {
              id: 'sr0',
              namespace: TalosRuntimeNamespace,
              type: TalosDiskType,
            },
            spec: {
              cdrom: true,
              dev_path: '/dev/sr0',
              size: 786432000,
              readonly: true,
              transport: 'ata',
            },
          },
          {
            // Empty CD-ROM: should be hidden from the view
            metadata: {
              id: 'sr1',
              namespace: TalosRuntimeNamespace,
              type: TalosDiskType,
            },
            spec: {
              cdrom: true,
              dev_path: '/dev/sr1',
              size: 0,
              readonly: true,
            },
          },
        ],
      }).handler,
      createWatchStreamHandler<DiscoveredVolumeSpec>({
        expectedOptions: {
          namespace: TalosRuntimeNamespace,
          type: TalosDiscoveredVolumeType,
        },
        initialResources: [
          {
            // Non-empty CDROM: name is set, so it is shown.
            metadata: {
              id: 'sr0',
              namespace: TalosRuntimeNamespace,
              type: TalosDiscoveredVolumeType,
            },
            spec: {
              dev_path: '/dev/sr0',
              name: 'iso9660',
              size: 786432000,
              type: 'disk',
            },
          },
          {
            // Empty CDROM: name is "" — Talos always creates the volume entry
            // but leaves name blank when no media is present. This disk should
            // be hidden from the view.
            metadata: {
              id: 'sr1',
              namespace: TalosRuntimeNamespace,
              type: TalosDiscoveredVolumeType,
            },
            spec: {
              dev_path: '/dev/sr1',
              name: '',
              size: 0,
              type: 'disk',
            },
          },
        ],
      }).handler,
      createWatchStreamHandler<VolumeStatusSpec>({
        expectedOptions: {
          namespace: TalosRuntimeNamespace,
          type: TalosVolumeStatusType,
        },
        initialResources: [],
      }).handler,
    )
  },
}

export const WithLuksEncryption: Story = {
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler<DiskSpec>({
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
              size: 21474836480,
              transport: 'virtio',
            },
          },
          {
            // Device-mapper device for the open LUKS container — should be
            // filtered out and NOT rendered as a top-level disk.
            metadata: {
              id: 'dm-0',
              namespace: TalosRuntimeNamespace,
              type: TalosDiskType,
            },
            spec: {
              dev_path: '/dev/dm-0',
              size: 104857600,
            },
          },
        ],
      }).handler,
      createWatchStreamHandler<DiscoveredVolumeSpec>({
        expectedOptions: {
          namespace: TalosRuntimeNamespace,
          type: TalosDiscoveredVolumeType,
        },
        initialResources: [
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
              size: 105906176,
              type: 'partition',
            },
          },
          {
            // LUKS-encrypted partition: name is 'luks2' in the raw discovered
            // volume, but VolumeStatus provides the inner filesystem (xfs).
            metadata: {
              id: 'vda2',
              namespace: TalosRuntimeNamespace,
              type: TalosDiscoveredVolumeType,
            },
            spec: {
              dev_path: '/dev/vda2',
              name: 'luks2',
              parent: 'vda',
              partition_index: 2,
              partition_label: 'EPHEMERAL',
              size: 20265148416,
              type: 'partition',
            },
          },
        ],
      }).handler,
      createWatchStreamHandler<VolumeStatusSpec>({
        expectedOptions: {
          namespace: TalosRuntimeNamespace,
          type: TalosVolumeStatusType,
        },
        initialResources: [
          {
            metadata: {
              id: 'EPHEMERAL',
              namespace: TalosRuntimeNamespace,
              type: TalosVolumeStatusType,
            },
            spec: {
              configuredEncryptionKeys: ['static'],
              encryptionProvider: 'luks2',
              encryptionSlot: 0,
              filesystem: 'xfs',
              location: '/dev/vda2',
              mountLocation: '/dev/dm-0',
              mountSpec: {
                targetPath: '/var',
              },
              parentLocation: '/dev/vda',
              partitionIndex: 2,
              phase: 'ready',
              prettySize: '19 GB',
              size: 20265148416,
              type: 'partition',
            },
          },
        ],
      }).handler,
    )
  },
}

export const NoData: Story = {
  beforeEach({ msw }) {
    msw.use(createWatchStreamHandler().handler)
  },
}
