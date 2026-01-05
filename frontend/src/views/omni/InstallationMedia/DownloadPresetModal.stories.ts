// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { dump } from 'js-yaml'
import { http, HttpResponse } from 'msw'
import { fn } from 'storybook/test'

import type { Resource } from '@/api/grpc'
import type {
  CreateSchematicRequest,
  CreateSchematicResponse,
} from '@/api/omni/management/management.pb'
import type { GetRequest, GetResponse } from '@/api/omni/resources/resources.pb'
import type { FeaturesConfigSpec, InstallationMediaConfigSpec } from '@/api/omni/specs/omni.pb'
import {
  type PlatformConfigSpec,
  PlatformConfigSpecArch,
  PlatformConfigSpecBootMethod,
} from '@/api/omni/specs/virtual.pb'
import {
  DefaultNamespace,
  FeaturesConfigID,
  FeaturesConfigType,
  InstallationMediaConfigType,
  LabelsMeta,
  MetalPlatformConfigType,
  PlatformMetalID,
  VirtualNamespace,
} from '@/api/resources'

import DownloadPresetModal from './DownloadPresetModal.vue'

const meta: Meta<typeof DownloadPresetModal> = {
  component: DownloadPresetModal,
  args: {
    open: true,
    onClose: fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default = {
  parameters: {
    msw: {
      handlers: [
        http.post<never, GetRequest, GetResponse>(
          '/omni.resources.ResourceService/Get',
          async ({ request }) => {
            const { type, namespace } = await request.clone().json()

            if (type !== InstallationMediaConfigType || namespace !== DefaultNamespace) return

            return HttpResponse.json({
              body: JSON.stringify({
                metadata: {},
                spec: {},
              } satisfies Resource<InstallationMediaConfigSpec>),
            })
          },
        ),

        http.post<never, CreateSchematicRequest, CreateSchematicResponse>(
          '/management.ManagementService/CreateSchematic',
          () => {
            const schematic_id = faker.string.uuid()

            return HttpResponse.json({
              schematic_id,
              pxe_url: `https://pxe.factory.talos.dev/pxe/${schematic_id}/1.12.0/metal-arm64-secureboot`,
              schematic_yml: dump(
                {
                  customization: {
                    extraKernelArgs: [
                      `siderolink.api=grpc://192.168.1.175:8090?grpc_tunnel=true&jointoken=mysecretjointoken`,
                      'talos.events.sink=[fdae:41e4:649b:9303::1]:8091',
                      'talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092',
                      '-console',
                      'console=tty0',
                    ],
                    meta: [{ key: LabelsMeta, value: 'machineLabels: {}\n' }],
                  },
                },
                { lineWidth: Number.MAX_SAFE_INTEGER },
              ),
            })
          },
        ),

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
                image_factory_pxe_base_url: 'https://pxe.factory.talos.dev',
              },
              metadata: {
                namespace: DefaultNamespace,
                type: FeaturesConfigType,
                id: FeaturesConfigID,
              },
            },
          ],
        }).handler,

        http.post<never, GetRequest, GetResponse>(
          '/omni.resources.ResourceService/Get',
          async ({ request }) => {
            const { id, type, namespace } = await request.clone().json()

            if (
              id !== PlatformMetalID ||
              type !== MetalPlatformConfigType ||
              namespace !== VirtualNamespace
            )
              return

            return HttpResponse.json({
              body: JSON.stringify({
                metadata: {
                  namespace: VirtualNamespace,
                  type: MetalPlatformConfigType,
                  id: PlatformMetalID,
                },
                spec: {
                  label: 'Bare Metal',
                  description: 'Runs on bare-metal servers',
                  architectures: [PlatformConfigSpecArch.AMD64, PlatformConfigSpecArch.ARM64],
                  documentation: '/talos-guides/install/bare-metal-platforms/',
                  disk_image_suffix: 'raw.zst',
                  boot_methods: [
                    PlatformConfigSpecBootMethod.ISO,
                    PlatformConfigSpecBootMethod.DISK_IMAGE,
                    PlatformConfigSpecBootMethod.PXE,
                  ],
                  secure_boot_supported: true,
                },
              } satisfies Resource<PlatformConfigSpec>),
            })
          },
        ),
      ],
    },
  },
} satisfies Story
