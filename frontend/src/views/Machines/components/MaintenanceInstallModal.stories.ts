// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { delay, http, HttpResponse } from 'msw'
import { compare, minor } from 'semver'

import { Code } from '@/api/google/rpc/code.pb.ts'
import type { Resource } from '@/api/grpc'
import type { MaintenanceLifecycleResponse } from '@/api/omni/management/management.pb.ts'
import type { MachineStatusSpec, TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, MachineStatusType, TalosVersionType } from '@/api/resources'

import MaintenanceInstallModal from './MaintenanceInstallModal.vue'

faker.seed(0)

const machineId = faker.string.uuid()

const versions = faker.helpers
  .uniqueArray(
    () => `1.${faker.number.int({ min: 28, max: 32 })}.${faker.number.int({ min: 0, max: 10 })}`,
    40,
  )
  .sort(compare)

const talosVersions = versions.map<Resource<TalosVersionSpec>>((version) => ({
  spec: {
    version,
    deprecated: faker.datatype.boolean(),
    unsupported: faker.datatype.boolean(),
    upgradable_talos_versions: versions.filter((v) => minor(v) === minor(version) + 1),
  },
  metadata: {
    id: version,
    type: TalosVersionType,
    namespace: DefaultNamespace,
  },
}))

const meta: Meta<typeof MaintenanceInstallModal> = {
  component: MaintenanceInstallModal,
  args: {
    open: true,
    machineId,
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Data = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<TalosVersionSpec>({
          expectedOptions: {
            type: TalosVersionType,
            namespace: DefaultNamespace,
          },
          initialResources: talosVersions,
        }).handler,

        createWatchStreamHandler<MachineStatusSpec>({
          expectedOptions: {
            type: MachineStatusType,
            namespace: DefaultNamespace,
            id: machineId,
          },
          initialResources: [
            {
              spec: {
                talos_version: `v${talosVersions.filter((s) => !s.spec.deprecated).at(-(talosVersions.length / 4))?.spec.version}`,
                hardware: {
                  blockdevices: [
                    {
                      linux_name: '/dev/sda',
                      model: 'QEMU HARDDISK',
                      size: '107374182400',
                      type: 'HDD',
                    },
                    {
                      linux_name: '/dev/sdb',
                      model: 'QEMU HARDDISK',
                      size: '53687091200',
                      type: 'SSD',
                    },
                  ],
                },
              },
              metadata: {
                type: MachineStatusType,
                namespace: DefaultNamespace,
                id: machineId,
              },
            },
          ],
        }).handler,

        http.post('/management.ManagementService/MaintenanceLifecycle', () => {
          faker.seed(42)

          const enc = new TextEncoder()

          const stream = new ReadableStream<Uint8Array>({
            async start(c) {
              await delay(1_000)

              for (let i = 0; i < 25; i++) {
                const response: MaintenanceLifecycleResponse = {
                  message: faker.hacker.phrase(),
                }

                await delay(faker.number.int({ min: 50, max: 250 }))

                c.enqueue(enc.encode(JSON.stringify(response) + '\n'))
              }

              c.close()
            },
          })

          return new HttpResponse(stream, {
            headers: {
              'content-type': 'application/json',
              'Grpc-metadata-content-type': 'application/grpc',
            },
          })
        }),
      ],
    },
  },
} satisfies Story

export const AlreadyInProgress: Story = {
  parameters: {
    msw: {
      handlers: [
        http.post('/management.ManagementService/MaintenanceLifecycle', () =>
          HttpResponse.json(
            {
              error: {
                code: Code.FAILED_PRECONDITION,
                message: `a maintenance lifecycle operation is already in progress for machine "${machineId}`,
              },
            },
            { status: 400 },
          ),
        ),

        ...Data.parameters.msw.handlers,
      ],
    },
  },
}
