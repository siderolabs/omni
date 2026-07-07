// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers.ts'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import type { FeaturesConfigSpec } from '@/api/omni/specs/omni.pb.ts'
import type {
  DefaultJoinTokenSpec,
  SiderolinkAPIConfigSpec,
} from '@/api/omni/specs/siderolink.pb.ts'
import type { SysVersionSpec } from '@/api/omni/specs/system.pb.ts'
import {
  APIConfigType,
  ConfigID,
  DefaultJoinTokenID,
  DefaultJoinTokenType,
  DefaultNamespace,
  EphemeralNamespace,
  FeaturesConfigID,
  FeaturesConfigType,
  SysVersionID,
  SysVersionType,
} from '@/api/resources.ts'
import * as DownloadTalosctlModalStories from '@/views/Home/components/DownloadTalosctl.stories'

import HomeGeneralInformation from './HomeGeneralInformation.vue'

faker.seed(0)

const joinToken = faker.string.alphanumeric(44)

const meta: Meta<typeof HomeGeneralInformation> = {
  component: HomeGeneralInformation,
}

export default meta
type Story = StoryObj<typeof meta>

export const Default = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<FeaturesConfigSpec>({
          expectedOptions: {
            type: FeaturesConfigType,
            namespace: DefaultNamespace,
            id: FeaturesConfigID,
          },
          initialResources: [
            {
              spec: {
                audit_log_enabled: true,
              },
              metadata: {
                type: FeaturesConfigType,
                namespace: DefaultNamespace,
                id: FeaturesConfigID,
              },
            },
          ],
        }).handler,

        createWatchStreamHandler<SysVersionSpec>({
          expectedOptions: {
            type: SysVersionType,
            namespace: EphemeralNamespace,
            id: SysVersionID,
          },
          initialResources: [
            {
              spec: {
                backend_version: `v${faker.system.semver()}`,
              },
              metadata: {
                type: SysVersionType,
                namespace: EphemeralNamespace,
                id: SysVersionID,
              },
            },
          ],
        }).handler,

        createWatchStreamHandler<DefaultJoinTokenSpec>({
          expectedOptions: {
            type: DefaultJoinTokenType,
            namespace: DefaultNamespace,
            id: DefaultJoinTokenID,
          },
          initialResources: [
            {
              spec: {
                token_id: joinToken,
              },
              metadata: {
                type: DefaultJoinTokenType,
                namespace: DefaultNamespace,
                id: DefaultJoinTokenID,
              },
            },
          ],
        }).handler,

        createWatchStreamHandler<SiderolinkAPIConfigSpec>({
          expectedOptions: {
            type: APIConfigType,
            namespace: DefaultNamespace,
            id: ConfigID,
          },
          initialResources: [
            {
              spec: {
                machine_api_advertised_url: faker.internet.url(),
                wireguard_advertised_endpoint: `grpc://${faker.internet.ipv4({ cidrBlock: '172.20.0.0/24' })}:8090?jointoken=${joinToken}`,
              },
              metadata: {
                type: APIConfigType,
                namespace: DefaultNamespace,
                id: ConfigID,
              },
            },
          ],
        }).handler,

        ...DownloadTalosctlModalStories.Default.parameters.msw.handlers,
      ],
    },
  },
} satisfies Story
