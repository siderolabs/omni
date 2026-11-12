// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import {
  DefaultNamespace,
  MachineStatusType,
  TalosBlockDeviceType,
  TalosBlockMountStatusType,
  TalosDiscoveredVolumeType,
  TalosDiskType,
  TalosLVMLogicalVolumeStatusType,
  TalosLVMVolumeGroupStatusType,
  TalosMDArrayStatusType,
  TalosRuntimeNamespace,
  TalosSMARTStatusType,
  TalosStorageNamespace,
  TalosSystemDiskType,
  TalosVolumeStatusType,
} from '@/api/resources'
import {
  type DeviceSpec,
  type DiscoveredVolumeSpec,
  type DiskSpec,
  type MountStatusSpec,
  type SMARTStatusSpec,
  type SystemDiskSpec,
  type VolumeStatusSpec,
} from '@/api/talos/block.pb'
import type {
  StorageServiceStatfsRequest,
  StorageServiceStatfsResponse,
} from '@/api/talos/machine/storage.pb'
import type {
  LVMLogicalVolumeStatusSpec,
  LVMVolumeGroupStatusSpec,
  MDArrayStatusSpec,
} from '@/api/talos/storage.pb'

import {
  type CapturedMachine,
  talos114LuksLvm,
  talos114LvmRaid1,
  talos114MdRaid1Failed,
  talos115LuksLvmRaid1,
  talos115LvmRaid1Degraded,
  talos115Maintenance,
  talos115MdRaid1Rebuilding,
  talos115MdRaid1Smart,
} from './fixtures'
import NodeDisks from './MachineDisks.vue'

const machineId = 'machine'

const machineStatusHandler = (talosVersion: string) =>
  createWatchStreamHandler<MachineStatusSpec>({
    expectedOptions: { namespace: DefaultNamespace, type: MachineStatusType, id: machineId },
    initialResources: [
      {
        metadata: { id: machineId, namespace: DefaultNamespace, type: MachineStatusType },
        spec: { talos_version: talosVersion },
      },
    ],
  }).handler

// Each block device with the devices it is built from, e.g. ['md127', ['sda', 'sdb']].
const blockDevicesHandler = (devices: [id: string, secondaries: string[], spec?: DeviceSpec][]) =>
  createWatchStreamHandler<DeviceSpec>({
    expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosBlockDeviceType },
    initialResources: devices.map(([id, secondaries, spec]) => ({
      metadata: { id, namespace: TalosRuntimeNamespace, type: TalosBlockDeviceType },
      spec: { secondaries, ...spec },
    })),
  }).handler

// The disk Talos is installed on, if any.
const systemDisksHandler = (diskID?: string) =>
  createWatchStreamHandler<SystemDiskSpec>({
    expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosSystemDiskType },
    initialResources: diskID
      ? [
          {
            metadata: {
              id: 'system-disk',
              namespace: TalosRuntimeNamespace,
              type: TalosSystemDiskType,
            },
            spec: { diskID, devPath: `/dev/${diskID}` },
          },
        ]
      : [],
  }).handler

const logicalVolumesHandler = (volumes: LVMLogicalVolumeStatusSpec[]) =>
  createWatchStreamHandler<LVMLogicalVolumeStatusSpec>({
    expectedOptions: { namespace: TalosStorageNamespace, type: TalosLVMLogicalVolumeStatusType },
    initialResources: volumes.map((spec) => ({
      metadata: {
        id: spec.fullName!,
        namespace: TalosStorageNamespace,
        type: TalosLVMLogicalVolumeStatusType,
      },
      spec,
    })),
  }).handler

// Serves resources captured from a real machine. See fixtures/README.md.
// Where Talos volumes are mounted, and how full each filesystem is, by mount target.
const volumeUsageHandlers = (
  mountStatuses: Resource<MountStatusSpec>[],
  statfs: Record<string, StorageServiceStatfsResponse>,
) => [
  createWatchStreamHandler<MountStatusSpec>({
    expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosBlockMountStatusType },
    initialResources: mountStatuses,
  }).handler,
  http.post('/machine.StorageService/Statfs', async ({ request }) => {
    const { path = '' } = (await request.json()) as StorageServiceStatfsRequest

    return statfs[path] ? HttpResponse.json(statfs[path]) : HttpResponse.json({}, { status: 404 })
  }),
]

// A mocked machine's mounted Talos volumes: their mount statuses, and how full
// each is.
const mockedVolumeUsage = (
  mounts: { volumeID: string; source: string; target: string; size: number; used: number }[],
) =>
  volumeUsageHandlers(
    mounts.map(({ volumeID, source, target }) => ({
      metadata: { id: volumeID, namespace: TalosRuntimeNamespace, type: TalosBlockMountStatusType },
      spec: { spec: { volumeID }, source, target },
    })),
    Object.fromEntries(
      mounts.map(({ target, size, used }) => [
        target,
        { size: String(size), used: String(used), available: String(size - used) },
      ]),
    ),
  )

// A ready volume status, for a mocked volume Talos has mounted.
const readyVolumeStatus = ({ volumeID, source }: { volumeID: string; source: string }) => ({
  metadata: { id: volumeID, namespace: TalosRuntimeNamespace, type: TalosVolumeStatusType },
  spec: { phase: 'ready', location: source, mountLocation: source, filesystem: 'xfs' },
})

const smartStatusesHandler = (statuses: Resource<SMARTStatusSpec>[]) =>
  createWatchStreamHandler<SMARTStatusSpec>({
    expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosSMARTStatusType },
    initialResources: statuses,
  }).handler

const mdArraysHandler = (arrays: Resource<MDArrayStatusSpec>[]) =>
  createWatchStreamHandler<MDArrayStatusSpec>({
    expectedOptions: { namespace: TalosStorageNamespace, type: TalosMDArrayStatusType },
    initialResources: arrays,
  }).handler

const volumeGroupsHandler = (groups: Resource<LVMVolumeGroupStatusSpec>[]) =>
  createWatchStreamHandler<LVMVolumeGroupStatusSpec>({
    expectedOptions: { namespace: TalosStorageNamespace, type: TalosLVMVolumeGroupStatusType },
    initialResources: groups,
  }).handler

const capturedMachineHandlers = (fixture: CapturedMachine) => [
  machineStatusHandler(fixture.talosVersion),
  createWatchStreamHandler<DiskSpec>({
    expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiskType },
    initialResources: fixture.disks,
  }).handler,
  createWatchStreamHandler<DeviceSpec>({
    expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosBlockDeviceType },
    initialResources: fixture.blockDevices,
  }).handler,
  createWatchStreamHandler<DiscoveredVolumeSpec>({
    expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiscoveredVolumeType },
    initialResources: fixture.volumes,
  }).handler,
  createWatchStreamHandler<VolumeStatusSpec>({
    expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosVolumeStatusType },
    initialResources: fixture.volumeStatuses,
  }).handler,
  createWatchStreamHandler<LVMLogicalVolumeStatusSpec>({
    expectedOptions: { namespace: TalosStorageNamespace, type: TalosLVMLogicalVolumeStatusType },
    initialResources: fixture.logicalVolumes,
  }).handler,
  createWatchStreamHandler<SystemDiskSpec>({
    expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosSystemDiskType },
    initialResources: fixture.systemDisks ?? [],
  }).handler,
  smartStatusesHandler(fixture.smartStatuses ?? []),
  mdArraysHandler(fixture.mdArrays ?? []),
  volumeGroupsHandler(fixture.volumeGroups ?? []),
  ...volumeUsageHandlers(fixture.mountStatuses ?? [], fixture.statfs ?? {}),
]

const meta: Meta<typeof NodeDisks> = {
  component: NodeDisks,
  args: { machineId },
  // Defaults for the watches most stories don't care about. A story's own
  // handlers take precedence over these.
  beforeEach({ msw }) {
    msw.use(
      machineStatusHandler('v1.15.0'),
      blockDevicesHandler([]),
      logicalVolumesHandler([]),
      systemDisksHandler(),
      smartStatusesHandler([]),
      mdArraysHandler([]),
      volumeGroupsHandler([]),
      ...volumeUsageHandlers([], {}),
    )
  },
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
          dev_path: '/dev/vda5',
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
    msw.use(
      ...handlers,
      // EPHEMERAL is nearly full.
      ...mockedVolumeUsage([
        { volumeID: 'EPHEMERAL', source: '/dev/vda4', target: '/var', size: 3.9e9, used: 3.59e9 },
      ]),
    )
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
      // Mounted from the opened LUKS device.
      ...mockedVolumeUsage([
        { volumeID: 'EPHEMERAL', source: '/dev/dm-0', target: '/var', size: 20e9, used: 6e9 },
      ]),
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

// How full each mounted filesystem is, as a fraction: from nearly empty, to
// nearly full enough to warn, to full.
const manyPartitionsUsage: Record<string, number> = {
  EPHEMERAL: 0.4,
  'u-data': 0.78,
  'u-logs': 0.95,
  'u-cache': 0.1,
  'u-scratch': 0,
  'u-backups': 0.6,
}

const manyPartitionsMounted = manyPartitions.flatMap(([label, size], i) =>
  label in manyPartitionsUsage
    ? [
        {
          volumeID: label,
          source: `/dev/nvme0n1p${i + 1}`,
          target: label === 'EPHEMERAL' ? '/var' : `/var/mnt/${label.slice(2)}`,
          size: Math.round(size),
          used: Math.round(size * manyPartitionsUsage[label]),
        },
      ]
    : [],
)

export const WithManyPartitions: Story = {
  beforeEach({ msw }) {
    msw.use(
      ...mockedVolumeUsage(manyPartitionsMounted),
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
        initialResources: manyPartitionsMounted.map(readyVolumeStatus),
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

// Talos 1.12+ reports partition offsets, so free space shows where it is on disk.
const partitionGapsMounted = [
  {
    volumeID: 'EPHEMERAL',
    source: '/dev/vda1',
    target: '/var',
    size: 10 * GiB,
    used: 3 * GiB,
  },
  {
    volumeID: 'u-data',
    source: '/dev/vda2',
    target: '/var/mnt/data',
    size: 10 * GiB,
    used: 5.5 * GiB,
  },
]

export const WithPartitionGaps: Story = {
  beforeEach({ msw }) {
    msw.use(
      ...mockedVolumeUsage(partitionGapsMounted),
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
        initialResources: partitionGapsMounted.map(readyVolumeStatus),
      }).handler,
    )
  },
}

// Volumes in other phases than ready: u-scratch failed before it was placed on
// any disk, so it only shows as an alert; u-logs failed on its partition.
export const WithVolumeProblems: Story = {
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler<DiskSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiskType },
        initialResources: [diskResource('vda', { size: 40 * GiB, transport: 'virtio' })],
      }).handler,
      createWatchStreamHandler<DiscoveredVolumeSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiscoveredVolumeType },
        initialResources: [
          volumeResource('vda', { type: 'disk', name: 'gpt', size: 40 * GiB }),
          ...(
            [
              ['EPHEMERAL', 'xfs', 20 * GiB],
              ['u-data', 'xfs', 10 * GiB],
              ['u-logs', '', 5 * GiB],
            ] as const
          ).map(([label, name, size], i) =>
            volumeResource(`vda${i + 1}`, {
              type: 'partition',
              name,
              parent: 'vda',
              partition_index: i + 1,
              partition_label: label,
              size,
            }),
          ),
        ],
      }).handler,
      createWatchStreamHandler<VolumeStatusSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosVolumeStatusType },
        initialResources: [
          {
            metadata: { id: 'EPHEMERAL', namespace: TalosRuntimeNamespace },
            spec: { phase: 'ready', type: 'partition', location: '/dev/vda1', filesystem: 'xfs' },
          },
          {
            metadata: { id: 'u-data', namespace: TalosRuntimeNamespace },
            spec: { phase: 'provisioned', type: 'partition', location: '/dev/vda2' },
          },
          {
            metadata: { id: 'u-logs', namespace: TalosRuntimeNamespace },
            spec: {
              phase: 'failed',
              preFailPhase: 'provisioned',
              type: 'partition',
              location: '/dev/vda3',
              errorMessage: 'error formatting partition: mkfs.xfs: exit status 1',
            },
          },
          {
            metadata: { id: 'u-scratch', namespace: TalosRuntimeNamespace },
            spec: {
              phase: 'failed',
              preFailPhase: 'waiting',
              type: 'partition',
              errorMessage:
                'no disks matched for volume (1 matched selector): 1 have not enough space, 0 have wrong format, 0 have other issues',
            },
          },
        ],
      }).handler,
    )
  },
}

// An MD RAID1 array built from one partition on each disk, next to Talos's own
// partitions on the first disk.
const raidOnPartitionsMounted = [
  {
    volumeID: 'EPHEMERAL',
    source: '/dev/nvme0n1p1',
    target: '/var',
    size: 100 * GiB,
    used: 25 * GiB,
  },
  {
    volumeID: 'u-mirror',
    source: '/dev/md0',
    target: '/var/mnt/mirror',
    size: 128 * GiB,
    used: 64 * GiB,
  },
]

export const WithRaidOnPartitions: Story = {
  beforeEach({ msw }) {
    msw.use(
      ...mockedVolumeUsage(raidOnPartitionsMounted),
      createWatchStreamHandler<DiskSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiskType },
        initialResources: [
          diskResource('nvme0n1', { size: 256 * GiB, transport: 'nvme' }),
          diskResource('nvme1n1', { size: 256 * GiB, transport: 'nvme' }),
          diskResource('md0', { size: 128 * GiB, secondary_disks: ['nvme0n1', 'nvme1n1'] }),
        ],
      }).handler,
      createWatchStreamHandler<DiscoveredVolumeSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiscoveredVolumeType },
        initialResources: [
          volumeResource('nvme0n1', { type: 'disk', name: 'gpt', offset: 0, size: 256 * GiB }),
          volumeResource('nvme1n1', { type: 'disk', name: 'gpt', offset: 0, size: 256 * GiB }),
          volumeResource('nvme0n1p1', {
            type: 'partition',
            name: 'xfs',
            parent: 'nvme0n1',
            partition_index: 1,
            partition_label: 'EPHEMERAL',
            offset: 1024 ** 2,
            size: 100 * GiB,
          }),
          ...['nvme0n1', 'nvme1n1'].map((disk, i) =>
            volumeResource(`${disk}p${2 - i}`, {
              type: 'partition',
              name: 'linux_raid_member',
              parent: disk,
              partition_index: 2 - i,
              offset: i ? 1024 ** 2 : 120 * GiB,
              size: 128 * GiB,
            }),
          ),
          volumeResource('md0', {
            type: 'disk',
            name: 'xfs',
            label: 'data',
            offset: 0,
            size: 128 * GiB,
          }),
        ],
      }).handler,
      createWatchStreamHandler<VolumeStatusSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosVolumeStatusType },
        initialResources: raidOnPartitionsMounted.map(readyVolumeStatus),
      }).handler,
      blockDevicesHandler([['md0', ['nvme0n1p2', 'nvme1n1p1']]]),
    )
  },
}

// A SAN LUN reached through two paths, sdb and sdc, with a logical volume on it.
const multipathMounted = [
  {
    volumeID: 'u-san',
    source: '/dev/dm-3',
    target: '/var/mnt/san',
    size: 1024 * GiB,
    used: 724 * GiB,
  },
]

export const WithMultipath: Story = {
  beforeEach({ msw }) {
    msw.use(
      ...mockedVolumeUsage(multipathMounted),
      createWatchStreamHandler<DiskSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiskType },
        initialResources: [
          diskResource('sdb', { size: 2048 * GiB, transport: 'iscsi' }),
          diskResource('sdc', { size: 2048 * GiB, transport: 'iscsi' }),
          diskResource('dm-2', {
            size: 2048 * GiB,
            secondary_disks: ['sdb', 'sdc'],
            device_mapper_name: '3600a098038303053',
            device_mapper_kind: 'mpath',
          }),
          diskResource('dm-3', {
            size: 1024 * GiB,
            secondary_disks: ['dm-2'],
            device_mapper_name: 'san-volumes',
            device_mapper_kind: 'lvm',
          }),
        ],
      }).handler,
      createWatchStreamHandler<DiscoveredVolumeSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiscoveredVolumeType },
        initialResources: [
          ...['sdb', 'sdc', 'dm-2'].map((id) =>
            volumeResource(id, { type: 'disk', name: 'lvm2-pv', offset: 0, size: 2048 * GiB }),
          ),
          volumeResource('dm-3', {
            type: 'disk',
            name: 'xfs',
            label: 'volumes',
            offset: 0,
            size: 1024 * GiB,
          }),
        ],
      }).handler,
      createWatchStreamHandler<VolumeStatusSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosVolumeStatusType },
        initialResources: multipathMounted.map(readyVolumeStatus),
      }).handler,
      blockDevicesHandler([
        ['dm-2', ['sdb', 'sdc']],
        ['dm-3', ['dm-2']],
      ]),
    )
  },
}

// A Talos 1.15 QEMU machine in maintenance mode, as captured.
export const CapturedTalos115Maintenance: Story = {
  beforeEach({ msw }) {
    msw.use(...capturedMachineHandlers(talos115Maintenance))
  },
}

// A Talos 1.14 QEMU machine with an LVM RAID1 logical volume, data/data, over
// vdc and vdd, as captured. LVM builds it from hidden rmeta/rimage sub-volumes
// (dm-0..dm-3), which Talos reports as block devices but not as disks.
export const CapturedTalos114LvmRaid1: Story = {
  beforeEach({ msw }) {
    msw.use(...capturedMachineHandlers(talos114LvmRaid1))
  },
}

// A Talos 1.14 QEMU machine with two LUKS-encrypted volumes on vda, one holding
// the LVM physical volume behind vg0/data, as captured.
export const CapturedTalos114LuksLvm: Story = {
  beforeEach({ msw }) {
    msw.use(...capturedMachineHandlers(talos114LuksLvm))
  },
}

// A Talos 1.15 QEMU machine with LUKS volumes and LVM on LUKS on vda, an LVM
// RAID1 logical volume over sda and vdb, and a filesystem on each logical
// volume, as captured with how full each filesystem is. Talos 1.15 reports each
// device-mapper device's kind.
export const CapturedTalos115LuksLvmRaid1: Story = {
  beforeEach({ msw }) {
    msw.use(...capturedMachineHandlers(talos115LuksLvmRaid1))
  },
}

// A Talos 1.15 QEMU machine with emulated NVMe and SATA disks monitored by
// SMART, the NVMe disk failing, and an MD RAID1 array over sda and vdb, as
// captured. Its config also
// asks for an array over one disk, which waits for another, and a user volume
// on a disk that doesn't exist, which fails.
export const CapturedTalos115MdRaid1Smart: Story = {
  beforeEach({ msw }) {
    msw.use(...capturedMachineHandlers(talos115MdRaid1Smart))
  },
}

// The machine of CapturedTalos115MdRaid1Smart, rebuilding its array after a
// member was wiped, as captured.
export const CapturedTalos115MdRaid1Rebuilding: Story = {
  beforeEach({ msw }) {
    msw.use(...capturedMachineHandlers(talos115MdRaid1Rebuilding))
  },
}

// A Talos 1.15 QEMU machine whose LVM RAID1 volume group lost a physical
// volume, as captured. LVM leaves its logical volume inactive, so there is no
// device for it, and the problem is shown above the disks.
export const CapturedTalos115LvmRaid1Degraded: Story = {
  beforeEach({ msw }) {
    msw.use(...capturedMachineHandlers(talos115LvmRaid1Degraded))
  },
}

// A Talos 1.14 QEMU machine on which Talos failed to create an MD RAID1 array,
// because an LVM volume group still held its disks, as captured. The array has
// no device, so its error is shown above the disks.
export const CapturedTalos114MdRaid1Failed: Story = {
  beforeEach({ msw }) {
    msw.use(...capturedMachineHandlers(talos114MdRaid1Failed))
  },
}

// Health problems the captured machines don't show: an MD array running on one
// member, which Talos re-adds too quickly to capture, and a SATA disk failing an
// ATA SMART attribute.
export const WithHealthProblems: Story = {
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler<DiskSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiskType },
        initialResources: [
          diskResource('sda', { size: 500 * GiB, transport: 'sata', rotational: true }),
          diskResource('sdb', { size: 500 * GiB, transport: 'sata', rotational: true }),
          diskResource('md0', {
            size: 499.9 * GiB,
            // sdb dropped out of the array, and still carries its stale superblock.
            secondary_disks: ['sda'],
            symlinks: ['/dev/disk/by-id/md-name-talos:data'],
          }),
        ],
      }).handler,
      createWatchStreamHandler<DiscoveredVolumeSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiscoveredVolumeType },
        initialResources: [
          ...['sda', 'sdb'].map((id) =>
            volumeResource(id, {
              type: 'disk',
              name: 'linux_raid_member',
              offset: 0,
              size: 500 * GiB,
            }),
          ),
          volumeResource('md0', { type: 'disk', name: 'xfs', offset: 0, size: 499.9 * GiB }),
        ],
      }).handler,
      createWatchStreamHandler<VolumeStatusSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosVolumeStatusType },
        initialResources: [],
      }).handler,
      blockDevicesHandler([['md0', ['sda']]]),
      mdArraysHandler([
        {
          metadata: { id: 'data', namespace: TalosStorageNamespace, type: TalosMDArrayStatusType },
          spec: {
            device: '/dev/disk/by-id/md-name-talos:data',
            level: 'raid1',
            // How Talos reports a mirror that lost a member: two slots, one member.
            members: ['/dev/sda'],
            raidDevices: 2,
            status: 'ready',
            syncAction: 'idle',
            arrayState: 'clean',
          },
        },
      ]),
      smartStatusesHandler([
        {
          metadata: { id: 'sda', namespace: TalosRuntimeNamespace, type: TalosSMARTStatusType },
          spec: {
            dev_path: '/dev/sda',
            dev_type: 'sata',
            healthy: false,
            message: 'attribute Reallocated_Sector_Ct is below its threshold',
            temperature: 41,
            power_on_hours: 38211,
            power_state: 'active',
            attributes: [
              {
                id: 5,
                name: 'Reallocated_Sector_Ct',
                current: 3,
                worst: 3,
                threshold: 10,
                raw_value: 3912,
                failing: true,
              },
              {
                id: 9,
                name: 'Power_On_Hours',
                current: 57,
                worst: 57,
                threshold: 0,
                raw_value: 38211,
              },
            ],
          },
        },
        {
          metadata: { id: 'sdb', namespace: TalosRuntimeNamespace, type: TalosSMARTStatusType },
          spec: {
            dev_path: '/dev/sdb',
            dev_type: 'sata',
            healthy: true,
            temperature: 38,
            power_on_hours: 38190,
          },
        },
      ]),
    )
  },
}

export const NoData: Story = {
  beforeEach({ msw }) {
    msw.use(createWatchStreamHandler().handler)
  },
}
