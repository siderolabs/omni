// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import type { Resource } from '@/api/grpc'
import type { IdentitySpec } from '@/api/omni/specs/auth.pb'
import {
  DefaultNamespace,
  EphemeralNamespace,
  IdentityStatusType,
  IdentityType,
  LabelIdentityUserID,
  SAMLLabelPrefix,
} from '@/api/resources'

import Users from './Users.vue'

const meta: Meta<typeof Users> = {
  component: Users,
}

export default meta
type Story = StoryObj<typeof meta>

const userIds = faker.helpers.multiple(() => faker.string.uuid(), { count: 100 })

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<IdentitySpec>({
          expectedOptions: {
            type: IdentityStatusType,
            namespace: EphemeralNamespace,
          },
          totalResults: userIds.length,
          initialResources: ({ limit = 5, offset = 0 }) => {
            faker.seed(offset)

            return faker.helpers.multiple<Resource<IdentitySpec>>(
              (_, i) => ({
                spec: {
                  user_id: userIds[i + offset],
                },
                metadata: {
                  type: IdentityType,
                  namespace: DefaultNamespace,
                  id: faker.internet.email(),
                  labels: {
                    [LabelIdentityUserID]: userIds[i + offset],
                    ...faker.helpers
                      .multiple(() => `${SAMLLabelPrefix}${faker.company.buzzNoun()}`, {
                        count: { min: 0, max: 3 },
                      })
                      .reduce((prev, curr) => ({ ...prev, [curr]: '' }), {}),
                  },
                },
              }),
              { count: limit },
            )
          },
        }).handler,
      ],
    },
  },
}
