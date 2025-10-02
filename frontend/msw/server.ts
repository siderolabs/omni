// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { setupServer } from 'msw/node'

import { createWatchStreamHandler, type WatchStreamHandlerOptions } from './helpers'

export const server = setupServer()

export function createWatchStreamMock<T = unknown, S = unknown>(
  options?: WatchStreamHandlerOptions<T, S>,
) {
  const { handler, pushEvents, closeStream } = createWatchStreamHandler(options)

  server.use(handler)

  return { pushEvents, closeStream }
}
