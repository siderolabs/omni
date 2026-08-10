// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers'
import { delay, http, HttpResponse } from 'msw'

import type { CreateRequest, CreateResponse } from '@/api/omni/resources/resources.pb'
import type { InstallationMediaConfigSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, InstallationMediaConfigType } from '@/api/resources'

export const handlers = [
  createWatchStreamHandler<InstallationMediaConfigSpec>({
    expectedOptions: {
      namespace: DefaultNamespace,
      type: InstallationMediaConfigType,
    },
    initialResources: (a) => {
      if (a.id !== 'existing') return []

      return [
        {
          spec: {},
          metadata: {
            namespace: DefaultNamespace,
            type: InstallationMediaConfigType,
            id: 'existing',
          },
        },
      ]
    },
  }).handler,
  http.post<never, CreateRequest, CreateResponse>(
    '/omni.resources.ResourceService/Create',
    async ({ request }) => {
      const { resource } = await request.clone().json()

      if (!resource?.metadata) return

      const { type, namespace } = resource.metadata

      if (type !== InstallationMediaConfigType || namespace !== DefaultNamespace) return

      await delay(1_000)

      return HttpResponse.json({})
    },
  ),
]
