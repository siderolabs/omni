// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'

import type { GetRequest } from '@/api/omni/resources/resources.pb'

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
    http.post<never, GetRequest>('/omni.resources.ResourceService/Get', () => {
      return HttpResponse.json(
        {},
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
