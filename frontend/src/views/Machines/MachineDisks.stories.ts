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

const GiB = 1024 ** 3

const manyPartitions: [label: string, size: number][] = [
  ['EFI', 0.1 * GiB],
  ['BOOT', 1 * GiB],
  ['META', 0.001 * GiB],
  ['STATE', 0.1 * GiB],
  ['EPHEMERAL', 20 * GiB],
  ['u-data', 15 * GiB],
  ['u-logs', 8 * GiB],
  // Past the last series colour, these share it.
  ['u-cache', 4 * GiB],
  ['u-scratch', 2 * GiB],
  ['u-backups', 6 * GiB],
]

export const WithManyPartitions: Story = {
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
              id: 'nvme0n1',
              namespace: TalosRuntimeNamespace,
              type: TalosDiskType,
            },
            spec: {
              dev_path: '/dev/nvme0n1',
              size: 64 * GiB,
              transport: 'nvme',
            },
          },
        ],
      }).handler,
      createWatchStreamHandler<DiscoveredVolumeSpec>({
        expectedOptions: {
          namespace: TalosRuntimeNamespace,
          type: TalosDiscoveredVolumeType,
        },
        initialResources: manyPartitions.map(([label, size], i) => ({
          metadata: {
            id: `nvme0n1p${i + 1}`,
            namespace: TalosRuntimeNamespace,
            type: TalosDiscoveredVolumeType,
          },
          spec: {
            dev_path: `/dev/nvme0n1p${i + 1}`,
            name: label === 'EFI' ? 'vfat' : 'xfs',
            parent: 'nvme0n1',
            partition_index: i + 1,
            partition_label: label,
            size: Math.round(size),
            type: 'partition',
          },
        })),
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

const diskResource = (id: string, spec: DiskSpec) => ({
  metadata: { id, namespace: TalosRuntimeNamespace, type: TalosDiskType },
  spec: { dev_path: `/dev/${id}`, ...spec },
})

const volumeResource = (id: string, spec: DiscoveredVolumeSpec) => ({
  metadata: { id, namespace: TalosRuntimeNamespace, type: TalosDiscoveredVolumeType },
  spec: { dev_path: `/dev/${id}`, ...spec },
})

// Two whole SATA disks in an MD RAID1 array, which holds a filesystem directly.
export const WithRaid: Story = {
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler<DiskSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiskType },
        initialResources: [
          diskResource('sda', { size: 500 * GiB, transport: 'sata', rotational: true }),
          diskResource('sdb', { size: 500 * GiB, transport: 'sata', rotational: true }),
          diskResource('md127', { size: 499.9 * GiB, secondary_disks: ['sda', 'sdb'] }),
        ],
      }).handler,
      createWatchStreamHandler<DiscoveredVolumeSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiscoveredVolumeType },
        initialResources: [
          ...['sda', 'sdb'].map((id) =>
            volumeResource(id, {
              type: 'disk',
              name: 'linux_raid_member',
              label: 'talos:data',
              uuid: '6f3c2a51-0c4e-4b0e-8a57-3d1f4f2b9c11',
              offset: 0,
              size: 500 * GiB,
            }),
          ),
          volumeResource('md127', {
            type: 'disk',
            name: 'xfs',
            label: 'data',
            uuid: faker.string.uuid(),
            offset: 0,
            size: 499.9 * GiB,
          }),
        ],
      }).handler,
      createWatchStreamHandler<VolumeStatusSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosVolumeStatusType },
        initialResources: [],
      }).handler,
    )
  },
}

// A whole disk used as an LVM physical volume, with two logical volumes on it.
// The logical volumes are device-mapper disks, which this view does not show.
export const WithLvm: Story = {
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler<DiskSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiskType },
        initialResources: [
          diskResource('sdc', { size: 1024 * GiB, transport: 'sata', rotational: true }),
          diskResource('dm-0', {
            size: 600 * GiB,
            secondary_disks: ['sdc'],
            device_mapper_name: 'vg0-data',
            device_mapper_kind: 'lvm',
          }),
          diskResource('dm-1', {
            size: 200 * GiB,
            secondary_disks: ['sdc'],
            device_mapper_name: 'vg0-logs',
            device_mapper_kind: 'lvm',
          }),
        ],
      }).handler,
      createWatchStreamHandler<DiscoveredVolumeSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiscoveredVolumeType },
        initialResources: [
          volumeResource('sdc', {
            type: 'disk',
            name: 'lvm2-pv',
            uuid: faker.string.uuid(),
            offset: 0,
            size: 1024 * GiB,
          }),
          volumeResource('dm-0', {
            type: 'disk',
            name: 'xfs',
            label: 'data',
            offset: 0,
            size: 600 * GiB,
          }),
          volumeResource('dm-1', {
            type: 'disk',
            name: 'xfs',
            label: 'logs',
            offset: 0,
            size: 200 * GiB,
          }),
        ],
      }).handler,
      createWatchStreamHandler<VolumeStatusSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosVolumeStatusType },
        initialResources: [],
      }).handler,
    )
  },
}

// Talos 1.12+ reports partition offsets, so free space shows where it is on disk.
export const WithPartitionGaps: Story = {
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler<DiskSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiskType },
        initialResources: [diskResource('vda', { size: 40 * GiB, transport: 'virtio' })],
      }).handler,
      createWatchStreamHandler<DiscoveredVolumeSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiscoveredVolumeType },
        initialResources: [
          volumeResource('vda', { type: 'disk', name: 'gpt', offset: 0, size: 40 * GiB }),
          volumeResource('vda1', {
            type: 'partition',
            name: 'xfs',
            parent: 'vda',
            partition_index: 1,
            partition_label: 'EPHEMERAL',
            offset: 1024 ** 2,
            size: 10 * GiB,
          }),
          volumeResource('vda2', {
            type: 'partition',
            name: 'xfs',
            parent: 'vda',
            partition_index: 2,
            partition_label: 'u-data',
            offset: 20 * GiB,
            size: 10 * GiB,
          }),
        ],
      }).handler,
      createWatchStreamHandler<VolumeStatusSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosVolumeStatusType },
        initialResources: [],
      }).handler,
    )
  },
}

export const NoData: Story = {
  beforeEach({ msw }) {
    msw.use(createWatchStreamHandler().handler)
  },
}
