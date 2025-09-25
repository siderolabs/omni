// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'

import type { WatchRequest, WatchResponse } from '@/api/omni/resources/resources.pb'

import { encodeResponse } from './helpers'

export const server = setupServer()

export function createWatchStreamMock() {
  let { stream, controller } = createStream()

  server.use(
    http.post<never, WatchRequest>('/omni.resources.ResourceService/Watch', () => {
      return new HttpResponse(stream, {
        headers: {
          'content-type': 'application/json',
          'Grpc-metadata-content-type': 'application/grpc',
        },
      })
    }),
  )

  return {
    pushEvents(...events: WatchResponse[]) {
      events.forEach((event) => controller.enqueue(encodeResponse(event)))
    },
    closeStream(error?: Error) {
      if (error) {
        controller.error(error)
      } else {
        controller.close()
      }

      // Prepare the stream for the next request
      ;({ stream, controller } = createStream())
    },
  }
}

function createStream() {
  let controller: ReadableStreamDefaultController<Uint8Array> | undefined
  const stream = new ReadableStream<Uint8Array>({ start: (c) => (controller = c) })

  if (!controller) throw new Error('Stream controller not initialised')

  return { stream, controller }
}
