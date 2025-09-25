// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Resource } from '@/api/grpc'
import type { WatchResponse } from '@/api/omni/resources/resources.pb'
import { EventType } from '@/api/omni/resources/resources.pb'

export function encodeResponse(response: WatchResponse) {
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
