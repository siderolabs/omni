// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import AuditLogs, { type AuditLogEvent, type AuditLogMsg } from './audit-logs.vue'

const meta: Meta<typeof AuditLogs> = {
  component: AuditLogs,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [() => ({ template: '<div class="h-screen"><story/></div>' })],
}

export default meta
type Story = StoryObj<typeof meta>

const EVENT_TYPES: AuditLogEvent[] = [
  'k8s_access',
  'talos_access',
  'create',
  'destroy',
  'update_with_conflicts',
  'update',
  'teardown',
]

const RESOURCE_TYPES = [
  'Cluster',
  'Machine',
  'ClusterMachine',
  'MachineSet',
  'ConfigPatch',
  'TalosConfig',
]

function makeEntry(ts: number): AuditLogMsg {
  const hasUser = faker.datatype.boolean()

  return {
    event_ts: ts,
    event_type: faker.helpers.arrayElement(EVENT_TYPES),
    event_data: {
      session: {
        user_agent: faker.internet.userAgent(),
        ...(hasUser
          ? {
              role: faker.helpers.arrayElement(['Admin', 'Operator', 'Reader']),
              email: faker.internet.email(),
            }
          : {}),
      },
    },
    resource_type: faker.helpers.arrayElement(RESOURCE_TYPES),
    resource_id: faker.string.uuid(),
  }
}

function makeHandler(count: number) {
  return http.post('/management.ManagementService/ReadAuditLog', () => {
    faker.seed(42)

    const enc = new TextEncoder()
    const now = Date.now()

    const stream = new ReadableStream<Uint8Array>({
      start(c) {
        for (let i = 0; i < count; i++) {
          const b64 = btoa(JSON.stringify(makeEntry(now - i * 60_000)) + '\n')
          c.enqueue(enc.encode(JSON.stringify({ audit_log: b64 }) + '\n'))
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
  })
}

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [makeHandler(1000)],
    },
  },
}

export const Empty: Story = {
  parameters: {
    msw: {
      handlers: [makeHandler(0)],
    },
  },
}
