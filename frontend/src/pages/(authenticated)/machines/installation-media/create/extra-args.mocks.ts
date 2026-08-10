// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { GetRequest, GetResponse } from '@/api/omni/resources/resources.pb'
import type { SBCConfigSpec } from '@/api/omni/specs/virtual.pb'
import { SBCConfigType, VirtualNamespace } from '@/api/resources'

export const SBC: Resource<SBCConfigSpec> = {
  metadata: {
    namespace: VirtualNamespace,
    type: SBCConfigType,
    id: faker.string.uuid(),
  },
  spec: {
    label: faker.commerce.productName(),
    documentation: faker.helpers.maybe(() => faker.system.directoryPath()),
    min_version: faker.helpers.maybe(
      () => `1.${faker.number.int({ min: 6, max: 11 })}.${faker.number.int({ min: 0, max: 10 })}`,
    ),
  },
}

export const handlers = [
  http.post<never, GetRequest, GetResponse>(
    '/omni.resources.ResourceService/Get',
    async ({ request }) => {
      const { type, namespace } = await request.clone().json()

      if (type !== SBCConfigType || namespace !== VirtualNamespace) return

      return HttpResponse.json({
        body: JSON.stringify(SBC),
      })
    },
  ),
]
