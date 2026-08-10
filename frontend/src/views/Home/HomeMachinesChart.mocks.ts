// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers'

import type { MachineStatusMetricsSpec } from '@/api/omni/specs/omni.pb'
import {
  EphemeralNamespace,
  MachineStatusMetricsID,
  MachineStatusMetricsType,
} from '@/api/resources'

export const handlers = [
  createWatchStreamHandler<MachineStatusMetricsSpec>({
    expectedOptions: {
      namespace: EphemeralNamespace,
      type: MachineStatusMetricsType,
      id: MachineStatusMetricsID,
    },
    initialResources: [
      {
        spec: {
          allocated_machines_count: 34,
          connected_machines_count: 40,
          pending_machines_count: 2,
          registered_machines_count: 42,
        },
        metadata: {
          namespace: EphemeralNamespace,
          type: MachineStatusMetricsType,
          id: MachineStatusMetricsID,
        },
      },
    ],
  }).handler,
]
