// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'

import type { FeaturesConfigSpec } from '@/api/omni/specs/omni.pb'
import type { DefaultJoinTokenSpec, SiderolinkAPIConfigSpec } from '@/api/omni/specs/siderolink.pb'
import type { SysVersionSpec } from '@/api/omni/specs/system.pb'
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
} from '@/api/resources'
import { handlers as downloadTalosctlHandlers } from '@/views/Home/components/DownloadTalosctl.mocks'

const joinToken = faker.string.alphanumeric(44)

export const handlers = [
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
  ...downloadTalosctlHandlers,
]
