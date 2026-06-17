// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { delay, http, HttpResponse } from 'msw'
import { fn } from 'storybook/test'

import type { Resource } from '@/api/grpc.ts'
import type {
  CreateServiceAccountRequest,
  CreateServiceAccountResponse,
} from '@/api/omni/management/management.pb.ts'
import type {
  CreateRequest,
  CreateResponse,
  GetRequest,
  GetResponse,
} from '@/api/omni/resources/resources.pb.ts'
import type { AdvertisedEndpointsSpec } from '@/api/omni/specs/virtual.pb.ts'
import { AdvertisedEndpointsID, AdvertisedEndpointsType, VirtualNamespace } from '@/api/resources'

import InfraProviderSetupModal from './InfraProviderSetupModal.vue'

const meta: Meta<typeof InfraProviderSetupModal> = {
  component: InfraProviderSetupModal,
  args: {
    open: true,
    'onUpdate:open': fn(),
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        http.post<never, CreateRequest, CreateResponse>(
          '/omni.resources.ResourceService/Create',
          async () => {
            await delay()

            return HttpResponse.json({})
          },
        ),

        http.post<never, CreateServiceAccountRequest, CreateServiceAccountResponse>(
          '/management.ManagementService/CreateServiceAccount',
          async () => {
            await delay()

            return HttpResponse.json({})
          },
        ),

        http.post<never, GetRequest, GetResponse>(
          '/omni.resources.ResourceService/Get',
          async () => {
            await delay()

            return HttpResponse.json({
              body: JSON.stringify({
                spec: {
                  grpc_api_url: faker.internet.url(),
                },
                metadata: {
                  namespace: VirtualNamespace,
                  type: AdvertisedEndpointsType,
                  id: AdvertisedEndpointsID,
                },
              } satisfies Resource<AdvertisedEndpointsSpec>),
            })
          },
        ),
      ],
    },
  },
}
