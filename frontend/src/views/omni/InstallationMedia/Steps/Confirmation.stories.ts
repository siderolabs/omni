// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { dump } from 'js-yaml'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import {
  type CreateSchematicRequest,
  type CreateSchematicResponse,
} from '@/api/omni/management/management.pb'
import type { GetRequest, ListRequest } from '@/api/omni/resources/resources.pb'
import { type FeaturesConfigSpec, type InstallationMediaSpec } from '@/api/omni/specs/omni.pb'
import {
  type PlatformConfigSpec,
  PlatformConfigSpecArch,
  PlatformConfigSpecBootMethod,
  type SBCConfigSpec,
} from '@/api/omni/specs/virtual.pb'
import {
  CloudPlatformConfigType,
  DefaultNamespace,
  DefaultTalosVersion,
  EphemeralNamespace,
  EtcdBackupOverallStatusID,
  EtcdBackupOverallStatusType,
  FeaturesConfigID,
  FeaturesConfigType,
  InstallationMediaType,
  MetalPlatformConfigType,
  MetricsNamespace,
  SBCConfigType,
  VirtualNamespace,
} from '@/api/resources'
import type { TalosctlDownloadsResponse } from '@/methods/useTalosctlDownloads'

import Confirmation from './Confirmation.vue'

const meta: Meta<typeof Confirmation> = {
  component: Confirmation,
  args: {
    modelValue: {
      currentStep: 0,
      hardwareType: 'metal',
      machineArch: PlatformConfigSpecArch.ARM64,
      talosVersion: DefaultTalosVersion,
      machineUserLabels: {
        'my-label': { canRemove: true, value: 'my-value' },
      },
      systemExtensions: ['siderolabs/potato', 'siderolabs/tomato'],
      cmdline: '-console console=tty0',
      secureBoot: true,
      useGrpcTunnel: false,
      joinToken: faker.string.alphanumeric(44),
    },
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<FeaturesConfigSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: FeaturesConfigType,
            id: FeaturesConfigID,
          },
          initialResources: [
            {
              spec: {
                image_factory_base_url: 'https://factory.talos.dev',
              },
              metadata: {
                namespace: MetricsNamespace,
                type: EtcdBackupOverallStatusType,
                id: EtcdBackupOverallStatusID,
              },
            },
          ],
        }).handler,

        http.get('/talosctl/downloads', () => {
          const versions = Object.fromEntries(
            faker.helpers.multiple(faker.system.semver).map(
              (v) =>
                [
                  `v${v}`,
                  faker.helpers.multiple(faker.hacker.noun, { count: 5 }).map((name) => ({
                    name,
                    url: `https://github.com/siderolabs/talos/releases/download/v${v}/talosctl-${name}-${faker.helpers.arrayElement(['amd64', 'arm64'])}`,
                  })),
                ] as const,
            ),
          )

          return HttpResponse.json<TalosctlDownloadsResponse>({
            status: '',
            release_data: {
              default_version: '',
              available_versions: versions,
            },
          })
        }),

        http.post<never, ListRequest>(
          '/omni.resources.ResourceService/List',
          async ({ request }) => {
            const { type, namespace } = await request.clone().json()

            if (type !== CloudPlatformConfigType || namespace !== VirtualNamespace) return

            const items = faker.helpers.multiple<Resource<PlatformConfigSpec>>(
              () => ({
                metadata: {
                  namespace,
                  type,
                  id: faker.string.uuid(),
                },
                spec: {
                  label: faker.commerce.productName(),
                  description: faker.commerce.productDescription(),
                  documentation: faker.helpers.maybe(() => faker.system.directoryPath()),
                  architectures: faker.helpers.arrayElements(
                    faker.helpers.uniqueArray(
                      () => faker.helpers.enumValue(PlatformConfigSpecArch),
                      2,
                    ),
                    { min: 1, max: 2 },
                  ),
                  secure_boot_supported: faker.datatype.boolean(),
                  min_version: faker.helpers.maybe(
                    () =>
                      `1.${faker.number.int({ min: 6, max: 11 })}.${faker.number.int({ min: 0, max: 10 })}`,
                  ),
                },
              }),
              { count: 20 },
            )

            return HttpResponse.json({
              items: items.map((item) => JSON.stringify(item)),
              total: items.length,
            })
          },
        ),

        http.post<never, ListRequest>(
          '/omni.resources.ResourceService/List',
          async ({ request }) => {
            const { type, namespace } = await request.clone().json()

            if (type !== SBCConfigType || namespace !== VirtualNamespace) return

            const items = faker.helpers.multiple<Resource<SBCConfigSpec>>(
              () => ({
                metadata: {
                  namespace,
                  type,
                  id: faker.string.uuid(),
                },
                spec: {
                  label: faker.commerce.productName(),
                  documentation: faker.helpers.maybe(() => faker.system.directoryPath()),
                  min_version: faker.helpers.maybe(
                    () =>
                      `1.${faker.number.int({ min: 6, max: 11 })}.${faker.number.int({ min: 0, max: 10 })}`,
                  ),
                },
              }),
              { count: 20 },
            )

            return HttpResponse.json({
              items: items.map((item) => JSON.stringify(item)),
              total: items.length,
            })
          },
        ),

        http.post<never, GetRequest>('/omni.resources.ResourceService/Get', async ({ request }) => {
          const { id, type, namespace } = await request.clone().json()

          if (id !== 'metal' || type !== MetalPlatformConfigType || namespace !== VirtualNamespace)
            return

          return HttpResponse.json({
            body: JSON.stringify({
              metadata: {
                namespace,
                type,
                id,
              },
              spec: {
                label: 'Bare Metal',
                boot_methods: [
                  PlatformConfigSpecBootMethod.DISK_IMAGE,
                  PlatformConfigSpecBootMethod.ISO,
                  PlatformConfigSpecBootMethod.PXE,
                ],
                disk_image_suffix: 'raw.zst',
                documentation: '/talos-guides/install/bare-metal-platforms/',
              },
            } satisfies Resource<PlatformConfigSpec>),
          })
        }),

        http.post<never, ListRequest>(
          '/omni.resources.ResourceService/List',
          async ({ request }) => {
            const { type, namespace } = await request.clone().json()

            if (type !== InstallationMediaType || namespace !== EphemeralNamespace) return

            const items = faker.helpers.multiple<Resource<InstallationMediaSpec>>(
              () => ({
                metadata: {
                  namespace: EphemeralNamespace,
                  type: InstallationMediaType,
                  id: faker.string.uuid(),
                },
                spec: {
                  architecture: 'arm64',
                  profile: 'metal',
                },
              }),
              { count: 20 },
            )

            return HttpResponse.json({
              items: items.map((item) => JSON.stringify(item)),
              total: items.length,
            })
          },
        ),

        http.post<never, CreateSchematicRequest>(
          '/management.ManagementService/CreateSchematic',
          async ({ request }) => {
            const { secure_boot, talos_version, join_token } = await request.clone().json()

            const schematic_id = faker.string.uuid()

            return HttpResponse.json<CreateSchematicResponse>({
              schematic_id,
              pxe_url: `https://pxe.factory.talos.dev/pxe/${schematic_id}/${talos_version}/metal-arm64-${secure_boot ? '-secureboot' : ''}`,
              schematic_yml: dump(
                {
                  customization: {
                    extraKernelArgs: [
                      `siderolink.api=grpc://192.168.1.175:8090?grpc_tunnel=true&jointoken=${join_token}`,
                      'talos.events.sink=[fdae:41e4:649b:9303::1]:8091',
                      'talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092',
                      '-console',
                      'console=tty0',
                    ],
                    meta: [
                      {
                        key: 12,
                        value: 'machineLabels: {}\n',
                      },
                    ],
                  },
                },
                { lineWidth: Number.MAX_SAFE_INTEGER },
              ),
            })
          },
        ),
      ],
    },
  },
} satisfies Story
