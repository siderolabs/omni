// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { isMatch } from 'lodash'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { WatchResponse } from '@/api/omni/resources/resources.pb'
import type { WatchRequest } from '@/api/omni/resources/resources.pb'
import { EventType } from '@/api/omni/resources/resources.pb'

export interface WatchStreamHandlerOptions<T, S> {
  skipBootstrap?: boolean
  expectedOptions?: Partial<WatchRequest & { selectors?: Record<string, string> }>
  totalResults?: number
  initialResources?:
    | Resource<T, S>[]
    | ((options: WatchRequest & { selectors?: Record<string, string> }) => Resource<T, S>[])
}

export function createWatchStreamHandler<T = unknown, S = unknown>({
  skipBootstrap,
  expectedOptions = {},
  totalResults,
  initialResources = [],
}: WatchStreamHandlerOptions<T, S> = {}) {
  const controllerRef: { value?: ReadableStreamDefaultController<Uint8Array> } = {}

  const handler = http.post<never, WatchRequest>(
    '/omni.resources.ResourceService/Watch',
    async ({ request }) => {
      // Cloning to not block fallthrough requests
      const options = await request.clone().json()

      const selectorHeader = request.headers.get('Grpc-Metadata-selectors')
      const selectors = selectorHeader
        ? Object.fromEntries(selectorHeader.split(',').map((s) => s.split('=') as [string, string]))
        : undefined

      if (!isMatch({ ...options, selectors }, expectedOptions)) return

      const { stream, controller } = createStream()
      controllerRef.value = controller

      const { sort_by_field: sort } = options

      const resources = Array.isArray(initialResources)
        ? initialResources
        : initialResources({ ...options, selectors })

      if (sort) {
        resources.sort((a, b) => String(a.metadata[sort]).localeCompare(b.metadata[sort]))
      }

      const events = resources.map((r, i) => createCreatedEvent(r, i + 1))

      if (!skipBootstrap) {
        events.push(createBootstrapEvent(totalResults ?? resources.length))
      }

      events.forEach((event) => controller.enqueue(encodeResponse(event)))

      return new HttpResponse(stream, {
        headers: {
          'content-type': 'application/json',
          'Grpc-metadata-content-type': 'application/grpc',
        },
      })
    },
  )

  return { handler, pushEvents, closeStream }

  async function waitForController() {
    return new Promise<void>((resolve) => {
      const intervalId = setInterval(() => {
        if (!controllerRef.value) return

        clearInterval(intervalId)
        resolve()
      }, 5)
    })
  }

  async function pushEvents(...events: WatchResponse[]) {
    await waitForController()

    events.forEach((event) => controllerRef.value!.enqueue(encodeResponse(event)))
  }

  async function closeStream(error?: Error) {
    await waitForController()

    if (error) {
      controllerRef.value!.error(error)
    } else {
      controllerRef.value!.close()
    }

    delete controllerRef.value
  }
}

function createStream() {
  let controller: ReadableStreamDefaultController<Uint8Array> | undefined
  const stream = new ReadableStream<Uint8Array>({ start: (c) => (controller = c) })

  if (!controller) throw new Error('Stream controller not initialised')

  return { stream, controller }
}

function encodeResponse(response: WatchResponse) {
  return new TextEncoder().encode(JSON.stringify(response) + '\n')
}

function createWatchResponse(
  eventType: EventType,
  resource?: Resource,
  oldResource?: Resource,
  total?: number,
): WatchResponse {
  return {
    event: {
      event_type: eventType,
      resource: resource ? JSON.stringify(resource) : undefined,
      old: oldResource ? JSON.stringify(oldResource) : undefined,
    },
    total,
  }
}

export function createBootstrapEvent(total?: number) {
  return createWatchResponse(EventType.BOOTSTRAPPED, undefined, undefined, total)
}

export function createCreatedEvent<TSpec = unknown, TStatus = unknown>(
  resource: Resource<TSpec, TStatus>,
  total?: number,
) {
  return createWatchResponse(EventType.CREATED, resource, undefined, total)
}

export function createUpdatedEvent<TSpec = unknown, TStatus = unknown>(
  resource: Resource<TSpec, TStatus>,
  oldResource: Resource<TSpec, TStatus>,
  total?: number,
) {
  return createWatchResponse(EventType.UPDATED, resource, oldResource, total)
}

export function createDestroyedEvent<TSpec = unknown, TStatus = unknown>(
  resource: Resource<TSpec, TStatus>,
  total?: number,
) {
  return createWatchResponse(EventType.DESTROYED, resource, undefined, total)
}
