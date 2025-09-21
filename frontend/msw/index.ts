// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'

import type { Resource } from '../src/api/grpc'
import type { WatchResponse } from '../src/api/omni/resources/resources.pb'
import { EventType } from '../src/api/omni/resources/resources.pb'

export const server = setupServer()

export function createWatchStreamMock() {
  let { stream, controller } = createStream()

  server.use(
    http.post('/omni.resources.ResourceService/Watch', () => {
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
      events
        .map((event) => JSON.stringify(event) + '\n')
        .map((jsonLine) => new TextEncoder().encode(jsonLine))
        .forEach((buffer) => controller.enqueue(buffer))
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

function createWatchResponse(
  eventType: EventType,
  resource?: Resource,
  oldResource?: Resource,
): WatchResponse {
  return {
    event: {
      event_type: eventType,
      resource: resource ? JSON.stringify(resource) : undefined,
      old: oldResource ? JSON.stringify(oldResource) : undefined,
    },
  }
}

export function createBootstrapEvent() {
  return createWatchResponse(EventType.BOOTSTRAPPED)
}

export function createCreatedEvent<TSpec = unknown, TStatus = unknown>(
  resource: Resource<TSpec, TStatus>,
) {
  return createWatchResponse(EventType.CREATED, resource)
}

export function createUpdatedEvent<TSpec = unknown, TStatus = unknown>(
  resource: Resource<TSpec, TStatus>,
  oldResource: Resource<TSpec, TStatus>,
) {
  return createWatchResponse(EventType.UPDATED, resource, oldResource)
}

export function createDestroyedEvent<TSpec = unknown, TStatus = unknown>(
  resource: Resource<TSpec, TStatus>,
) {
  return createWatchResponse(EventType.DESTROYED, resource)
}
