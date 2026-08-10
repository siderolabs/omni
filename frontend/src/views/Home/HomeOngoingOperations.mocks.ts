// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers'

import type { OngoingTaskSpec } from '@/api/omni/specs/omni.pb'
import { EphemeralNamespace, OngoingTaskType } from '@/api/resources'

export const handlers = [
  createWatchStreamHandler<OngoingTaskSpec>({
    expectedOptions: {
      namespace: EphemeralNamespace,
      type: OngoingTaskType,
    },
    initialResources: [
      {
        metadata: {
          namespace: EphemeralNamespace,
          type: OngoingTaskType,
          id: 'prod-east-k8s',
          created: '2026-07-07T09:30:00Z',
        },
        spec: {
          title: 'prod-east',
          kubernetes_upgrade: {
            last_upgrade_version: '1.30.1',
            current_upgrade_version: '1.31.0',
          },
        },
      },
      {
        metadata: {
          namespace: EphemeralNamespace,
          type: OngoingTaskType,
          id: 'staging-talos',
          created: '2026-07-07T09:42:00Z',
        },
        spec: {
          title: 'staging',
          talos_upgrade: {
            last_upgrade_version: 'v1.7.5',
            current_upgrade_version: 'v1.8.0',
          },
        },
      },
    ],
  }).handler,
]
