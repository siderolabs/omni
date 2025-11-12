// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { getUnixTime } from 'date-fns'
import fetchIntercept, { type FetchInterceptor } from 'fetch-intercept'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import { defineComponent, ref } from 'vue'

import { useRegisterAPIInterceptor } from './interceptor'

vi.mock('fetch-intercept')
vi.mock('@/methods/identity', () => ({
  useIdentity() {
    return { identity: ref('testuser') }
  },
}))
vi.mock('@/methods/key', () => ({
  useKeys() {
    return {
      publicKeyID: ref('public_key_id'),
    }
  },
  useSignDetached() {
    return () => new ArrayBuffer(10)
  },
}))

const fetchInterceptMock = vi.mocked(fetchIntercept)

const TestComponent = defineComponent({
  setup: useRegisterAPIInterceptor,
  template: '<template />',
})

describe('useRegisterAPIInterceptor', () => {
  const now = 1762881349000

  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    vi.setSystemTime(now)
  })

  enableAutoUnmount(afterEach)

  afterEach(() => {
    vi.useRealTimers()
  })

  test.each(['/api', '/api/auth.AuthService/RevokePublicKey'])(
    'intercepts api route: %s',
    async (originalUrl) => {
      let interceptorMock: FetchInterceptor['request']

      fetchInterceptMock.register.mockImplementation(({ request }) => {
        interceptorMock = request

        return vi.fn()
      })

      mount(TestComponent)

      expect(fetchInterceptMock.register).toHaveBeenCalledOnce()
      expectToBeDefined(interceptorMock)

      const [url, config] = await interceptorMock(originalUrl, undefined)
      const headers = Array.from(config.headers) as [string, string]

      expect(url).toBe(originalUrl)
      expect(headers).toMatchObject([
        [
          'grpc-metadata-x-sidero-payload',
          JSON.stringify({
            headers: { 'x-sidero-timestamp': [getUnixTime(now).toString()] },
            method: originalUrl.replace('/api', ''),
          }),
        ],
        ['grpc-metadata-x-sidero-signature', 'siderov1 testuser public_key_id AAAAAAAAAAAAAA=='],
        ['grpc-metadata-x-sidero-timestamp', getUnixTime(now).toString()],
      ])
    },
  )

  test.each(['/image/', '/image/thing/whatever'])(
    'intercepts image route: %s',
    async (originalUrl) => {
      let interceptorMock: FetchInterceptor['request']

      fetchInterceptMock.register.mockImplementation(({ request }) => {
        interceptorMock = request

        return vi.fn()
      })

      mount(TestComponent)

      expect(fetchInterceptMock.register).toHaveBeenCalledOnce()
      expectToBeDefined(interceptorMock)

      const [url, config] = await interceptorMock(originalUrl, undefined)
      const headers = Array.from(config.headers) as [string, string]

      expect(url).toBe(originalUrl)
      expect(headers).toMatchObject([
        ['x-sidero-signature', 'siderov1 testuser public_key_id AAAAAAAAAAAAAA=='],
        ['x-sidero-timestamp', getUnixTime(now).toString()],
      ])
    },
  )

  test.each(['/fake', '/api/auth.whatever'])(
    'does not intercept route: %s',
    async (originalUrl) => {
      let interceptorMock: FetchInterceptor['request']

      fetchInterceptMock.register.mockImplementation(({ request }) => {
        interceptorMock = request

        return vi.fn()
      })

      mount(TestComponent)

      expect(fetchInterceptMock.register).toHaveBeenCalledOnce()
      expectToBeDefined(interceptorMock)

      const [url, config] = await interceptorMock(originalUrl, undefined)

      expect(url).toBe(originalUrl)
      expect(config).toBeUndefined()
    },
  )

  test('unregisters interceptor on unmount', async () => {
    const unregisterMock = vi.fn()

    fetchInterceptMock.register.mockImplementation(() => unregisterMock)

    const wrapper = mount(TestComponent)

    expect(unregisterMock).not.toHaveBeenCalled()
    wrapper.unmount()
    expect(unregisterMock).toHaveBeenCalledOnce()
  })
})

function expectToBeDefined<T>(value: T | undefined): asserts value is T {
  expect(value).toBeDefined()
}
