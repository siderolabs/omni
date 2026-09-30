// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { addDays, subDays, subHours } from 'date-fns'
import { delay, http, HttpResponse } from 'msw'
import { fn } from 'storybook/test'

import type { Empty } from '@/api/google/protobuf/empty.pb'
import type { RevokeServiceAccountKeyRequest } from '@/api/omni/management/management.pb'
import type { ServiceAccountStatusSpecPgpPublicKey } from '@/api/omni/specs/auth.pb'

import ServiceAccountRevokeKeyModal from './ServiceAccountRevokeKeyModal.vue'

const keyId = () => faker.string.hexadecimal({ length: 40, casing: 'lower', prefix: '' })

const revokedKeyId = keyId()

const fakeKey = (
  overrides: Partial<ServiceAccountStatusSpecPgpPublicKey> = {},
): ServiceAccountStatusSpecPgpPublicKey => ({
  id: keyId(),
  created: subDays(new Date(), 30).toISOString(),
  expiration: addDays(new Date(), 300).toISOString(),
  last_used: subDays(new Date(), 7).toISOString(),
  ...overrides,
})

const meta: Meta<typeof ServiceAccountRevokeKeyModal> = {
  component: ServiceAccountRevokeKeyModal,

  args: {
    open: true,
    'onUpdate:open': fn(),
    identity: 'automation@serviceaccount.omni.sidero.dev',
    publicKeyId: revokedKeyId,
    keys: [fakeKey({ id: revokedKeyId }), fakeKey()],
  },

  beforeEach({ msw }) {
    msw.use(
      http.post<never, RevokeServiceAccountKeyRequest, Empty>(
        '/management.ManagementService/RevokeServiceAccountKey',
        async () => {
          await delay()

          return HttpResponse.json({})
        },
      ),
    )
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const RecentlyUsed: Story = {
  args: {
    keys: [
      fakeKey({ id: revokedKeyId, last_used: subHours(new Date(), 2).toISOString() }),
      fakeKey(),
    ],
  },
}

export const LastValidKey: Story = {
  args: {
    keys: [
      fakeKey({ id: revokedKeyId }),
      fakeKey({ expiration: subDays(new Date(), 1).toISOString() }),
    ],
  },
}
