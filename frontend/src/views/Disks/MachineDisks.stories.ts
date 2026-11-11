// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import {
  DefaultNamespace,
  MachineStatusType,
  TalosBlockDeviceType,
  TalosDiscoveredVolumeType,
  TalosDiskType,
  TalosLVMLogicalVolumeStatusType,
  TalosRuntimeNamespace,
  TalosStorageNamespace,
  TalosSystemDiskType,
  TalosVolumeStatusType,
} from '@/api/resources'
import {
  type DeviceSpec,
  type DiscoveredVolumeSpec,
  type DiskSpec,
  type SystemDiskSpec,
  type VolumeStatusSpec,
} from '@/api/talos/block.pb'
import type { LVMLogicalVolumeStatusSpec } from '@/api/talos/storage.pb'

import {
  type CapturedMachine,
  talos114LuksLvm,
  talos114LvmRaid1,
  talos114Maintenance,
  talos114MdRaid1,
  talos115LuksLvmRaid1,
  talos115MdRaid1,
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

// An NVMe system disk and a SATA hard disk, with the identifiers Talos reports
// for real hardware.
export const WithDiskDetails: Story = {
  beforeEach({ msw }) {
    msw.use(
      systemDisksHandler('nvme0n1'),
      createWatchStreamHandler<DiskSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiskType },
        initialResources: [
          diskResource('nvme0n1', {
            size: 1000 * GiB,
            transport: 'nvme',
            rotational: false,
            model: 'Samsung SSD 990 PRO 1TB',
            serial: 'S73VNJ0W123456X',
            firmware_version: '4B2QJXD7',
            wwid: 'eui.002538b231a0b4c1',
            bus_path: '/pci0000:00/0000:00:1d.0/0000:03:00.0/nvme/nvme0/nvme0n1',
            symlinks: [
              '/dev/disk/by-diskseq/1',
              '/dev/disk/by-id/nvme-Samsung_SSD_990_PRO_1TB_S73VNJ0W123456X',
              '/dev/disk/by-id/nvme-eui.002538b231a0b4c1',
              '/dev/disk/by-path/pci-0000:03:00.0-nvme-1',
            ],
          }),
          diskResource('sda', {
            size: 4000 * GiB,
            transport: 'sata',
            rotational: true,
            model: 'WDC WD40EFPX-68C',
            serial: 'WD-WX12D3456789',
            wwid: 'naa.50014ee2b1234567',
            bus_path: '/pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0',
            symlinks: [
              '/dev/disk/by-id/ata-WDC_WD40EFPX-68C_WD-WX12D3456789',
              '/dev/disk/by-id/wwn-0x50014ee2b1234567',
              '/dev/disk/by-path/pci-0000:00:17.0-ata-1',
            ],
          }),
        ],
      }).handler,
      createWatchStreamHandler<DiscoveredVolumeSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiscoveredVolumeType },
        initialResources: [
          volumeResource('nvme0n1', { type: 'disk', name: 'gpt', offset: 0, size: 1000 * GiB }),
          volumeResource('nvme0n1p1', {
            type: 'partition',
            name: 'vfat',
            parent: 'nvme0n1',
            partition_index: 1,
            partition_label: 'EFI',
            offset: 1024 ** 2,
            size: GiB,
          }),
          volumeResource('nvme0n1p2', {
            type: 'partition',
            name: 'xfs',
            parent: 'nvme0n1',
            partition_index: 2,
            partition_label: 'EPHEMERAL',
            offset: GiB + 1024 ** 2,
            size: 999 * GiB - 2 * 1024 ** 2,
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
      blockDevicesHandler([['md127', ['sda', 'sdb']]]),
    )
  },
}

// A whole disk used as an LVM physical volume, with two logical volumes on it.
// Talos 1.15+ reports the logical volumes' device-mapper kind and name.
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
      blockDevicesHandler([
        ['dm-0', ['sdc']],
        ['dm-1', ['sdc']],
      ]),
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

// The same as WithLvm on Talos 1.14, which doesn't report the device-mapper
// kind: the logical volumes are recognised by their kernel device numbers.
export const WithLvmOnTalos114: Story = {
  beforeEach({ msw }) {
    msw.use(
      machineStatusHandler('v1.14.2'),
      createWatchStreamHandler<DiskSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiskType },
        initialResources: [
          diskResource('sdc', { size: 1024 * GiB, transport: 'sata', rotational: true }),
          diskResource('dm-0', { size: 600 * GiB, secondary_disks: ['sdc'] }),
          diskResource('dm-1', { size: 200 * GiB, secondary_disks: ['sdc'] }),
        ],
      }).handler,
      createWatchStreamHandler<DiscoveredVolumeSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiscoveredVolumeType },
        initialResources: [
          volumeResource('sdc', { type: 'disk', name: 'lvm2-pv', offset: 0, size: 1024 * GiB }),
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
      blockDevicesHandler([
        ['dm-0', ['sdc'], { major: 253, minor: 0 }],
        ['dm-1', ['sdc'], { major: 253, minor: 1 }],
      ]),
      logicalVolumesHandler([
        { fullName: 'vg0/data', kernelMajor: '253', kernelMinor: '0', layout: 'linear' },
        { fullName: 'vg0/logs', kernelMajor: '253', kernelMinor: '1', layout: 'linear' },
      ]),
    )
  },
}

// An MD RAID1 array built from one partition on each disk, next to Talos's own
// partitions on the first disk.
export const WithRaidOnPartitions: Story = {
  beforeEach({ msw }) {
    msw.use(
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
        initialResources: [],
      }).handler,
      blockDevicesHandler([['md0', ['nvme0n1p2', 'nvme1n1p1']]]),
    )
  },
}

// A LUKS-encrypted partition holding an LVM physical volume. The decrypted
// device (dm-0) is not shown: the logical volume is backed by the disk.
export const WithLvmOnLuks: Story = {
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler<DiskSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiskType },
        initialResources: [
          diskResource('sda', { size: 512 * GiB, transport: 'sata' }),
          diskResource('dm-0', {
            size: 499 * GiB,
            secondary_disks: ['sda'],
            device_mapper_name: 'luks-data',
            device_mapper_kind: 'crypt',
          }),
          diskResource('dm-1', {
            size: 400 * GiB,
            secondary_disks: ['dm-0'],
            device_mapper_name: 'vg0-data',
            device_mapper_kind: 'lvm',
          }),
        ],
      }).handler,
      createWatchStreamHandler<DiscoveredVolumeSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosDiscoveredVolumeType },
        initialResources: [
          volumeResource('sda', { type: 'disk', name: 'gpt', offset: 0, size: 512 * GiB }),
          volumeResource('sda1', {
            type: 'partition',
            name: 'xfs',
            parent: 'sda',
            partition_index: 1,
            partition_label: 'EPHEMERAL',
            offset: 1024 ** 2,
            size: 12 * GiB,
          }),
          volumeResource('sda2', {
            type: 'partition',
            name: 'luks',
            parent: 'sda',
            partition_index: 2,
            offset: 13 * GiB,
            size: 499 * GiB,
          }),
          volumeResource('dm-0', { type: 'disk', name: 'lvm2-pv', offset: 0, size: 499 * GiB }),
          volumeResource('dm-1', {
            type: 'disk',
            name: 'xfs',
            label: 'data',
            offset: 0,
            size: 400 * GiB,
          }),
        ],
      }).handler,
      createWatchStreamHandler<VolumeStatusSpec>({
        expectedOptions: { namespace: TalosRuntimeNamespace, type: TalosVolumeStatusType },
        initialResources: [],
      }).handler,
      blockDevicesHandler([
        ['dm-0', ['sda2']],
        ['dm-1', ['dm-0']],
      ]),
    )
  },
}

// A SAN LUN reached through two paths, sdb and sdc, with a logical volume on it.
export const WithMultipath: Story = {
  beforeEach({ msw }) {
    msw.use(
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
        initialResources: [],
      }).handler,
      blockDevicesHandler([
        ['dm-2', ['sdb', 'sdc']],
        ['dm-3', ['dm-2']],
      ]),
    )
  },
}

// A Talos 1.14 QEMU machine in maintenance mode, as captured.
export const CapturedTalos114Maintenance: Story = {
  beforeEach({ msw }) {
    msw.use(...capturedMachineHandlers(talos114Maintenance))
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

// A Talos 1.14 QEMU machine with an MD RAID1 array, md0, over vdc and vdd, and
// a user volume on the array, as captured.
export const CapturedTalos114MdRaid1: Story = {
  beforeEach({ msw }) {
    msw.use(...capturedMachineHandlers(talos114MdRaid1))
  },
}

// A Talos 1.14 QEMU machine with two LUKS-encrypted volumes on vda, one holding
// the LVM physical volume behind vg0/data, as captured.
export const CapturedTalos114LuksLvm: Story = {
  beforeEach({ msw }) {
    msw.use(...capturedMachineHandlers(talos114LuksLvm))
  },
}

// A Talos 1.15 QEMU machine with an MD RAID1 array, as captured.
export const CapturedTalos115MdRaid1: Story = {
  beforeEach({ msw }) {
    msw.use(...capturedMachineHandlers(talos115MdRaid1))
  },
}

// A Talos 1.15 QEMU machine with LUKS volumes and LVM on LUKS on vda, and an
// LVM RAID1 logical volume over vdc and vdd, as captured. Talos 1.15 reports
// each device-mapper device's kind.
export const CapturedTalos115LuksLvmRaid1: Story = {
  beforeEach({ msw }) {
    msw.use(...capturedMachineHandlers(talos115LuksLvmRaid1))
  },
}

export const NoData: Story = {
  beforeEach({ msw }) {
    msw.use(createWatchStreamHandler().handler)
  },
}
