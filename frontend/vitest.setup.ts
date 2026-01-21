// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import '@testing-library/jest-dom/vitest'
import 'fake-indexeddb/auto'

import { server } from '@msw/server'
import { cleanup } from '@testing-library/vue'
import { afterAll, afterEach, beforeAll, vi } from 'vitest'

beforeAll(() => server.listen({ onUnhandledRequest: 'bypass' }))
afterEach(() => server.resetHandlers())
afterAll(() => server.close())

/**
 * Required for testing-library cleanup, in place of globals
 * See: https://testing-library.com/docs/vue-testing-library/setup/#cleanup
 */
afterEach(() => cleanup())

// Used by reka-ui select, tests fail without it
window.HTMLElement.prototype.hasPointerCapture = vi.fn()
