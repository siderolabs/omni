// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { ReadAuditLogRequest, ReadAuditLogResponse } from '@/api/omni/management/management.pb'

import DownloadAuditLogsModal from './DownloadAuditLogsModal.vue'

const meta: Meta<typeof DownloadAuditLogsModal> = {
  component: DownloadAuditLogsModal,
  args: {
    open: true,
  },
  parameters: {
    layout: 'fullscreen',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default = {
  parameters: {
    msw: {
      handlers: [
        http.post<never, ReadAuditLogRequest>('/management.ManagementService/ReadAuditLog', () => {
          const stream = new ReadableStream<Uint8Array>({
            async start(c) {
              faker.seed(0)

              const enc = new TextEncoder()

              faker.helpers
                .multiple(
                  () => ({
                    audit_log: {
                      event_type: 'create',
                      resource_type: faker.internet.domainName(),
                      resource_id: faker.internet.email(),
                      event_data: {
                        new_user: {
                          id: faker.string.uuid(),
                          email: faker.internet.email(),
                        },
                        session: { user_agent: faker.internet.userAgent() },
                      },
                      event_ts: faker.date.recent().valueOf(),
                    },
                  }),
                  { count: 10_000 },
                )
                .sort((a, b) => a.audit_log.event_ts - b.audit_log.event_ts)
                .map<ReadAuditLogResponse>((a) => ({
                  ...a,
                  audit_log: btoa(JSON.stringify(a.audit_log) + '\n') as unknown as Uint8Array,
                }))
                .forEach((l) => c.enqueue(enc.encode(JSON.stringify(l) + '\n')))

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
