// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'

import type { Resource } from '@/api/grpc'
import type { GetRequest, GetResponse } from '@/api/omni/resources/resources.pb'

import { createWatchStreamHandler, type WatchStreamHandlerOptions } from './helpers'

export const server = setupServer()

export function createWatchStreamMock<T = unknown, S = unknown>(
  options?: WatchStreamHandlerOptions<T, S>,
) {
  const { handler, pushEvents, closeStream } = createWatchStreamHandler(options)

  server.use(handler)

  return { pushEvents, closeStream }
}

export function createGetMock() {
  server.use(
    http.post<never, GetRequest, GetResponse>('/omni.resources.ResourceService/Get', () => {
      return HttpResponse.json(
        { body: JSON.stringify({ spec: {}, metadata: {} } satisfies Resource) },
        {
          headers: {
            'content-type': 'application/json',
            'Grpc-metadata-content-type': 'application/grpc',
          },
        },
      )
    }),
  )
}
