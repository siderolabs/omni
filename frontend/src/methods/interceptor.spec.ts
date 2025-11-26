// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { getUnixTime } from 'date-fns'
import fetchIntercept, { type FetchInterceptor } from 'fetch-intercept'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import { defineComponent, ref } from 'vue'

import { KeysError, KeysErrorCode, useSignDetached } from '@/methods/key'

import { useRegisterAPIInterceptor } from './interceptor'

vi.mock('fetch-intercept')
vi.mock('@/methods/identity', () => ({
  useIdentity: vi.fn(() => ({ identity: ref('testuser') })),
}))
vi.mock(import('@/methods/key'), async (importOriginal) => {
  const original = await importOriginal()
  return {
    ...original,
    useKeys: vi.fn(
      () => ({ publicKeyID: ref('public_key_id') }) as ReturnType<(typeof original)['useKeys']>,
    ),
    useSignDetached: vi.fn(() => async () => new ArrayBuffer(10)),
  }
})

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
      let interceptor: FetchInterceptor['request']

      fetchInterceptMock.register.mockImplementation(({ request }) => {
        interceptor = request

        return vi.fn()
      })

      mount(TestComponent)

      expect(fetchInterceptMock.register).toHaveBeenCalledOnce()
      expectToBeDefined(interceptor)

      const [url, config] = await interceptor(originalUrl, undefined)
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
      let interceptor: FetchInterceptor['request']

      fetchInterceptMock.register.mockImplementation(({ request }) => {
        interceptor = request

        return vi.fn()
      })

      mount(TestComponent)

      expect(fetchInterceptMock.register).toHaveBeenCalledOnce()
      expectToBeDefined(interceptor)

      const [url, config] = await interceptor(originalUrl, undefined)
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
      let interceptor: FetchInterceptor['request']

      fetchInterceptMock.register.mockImplementation(({ request }) => {
        interceptor = request

        return vi.fn()
      })

      mount(TestComponent)

      expect(fetchInterceptMock.register).toHaveBeenCalledOnce()
      expectToBeDefined(interceptor)

      const [url, config] = await interceptor(originalUrl, undefined)

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

  test('waits for keys to be ready', async () => {
    let interceptor: FetchInterceptor['request']

    fetchInterceptMock.register.mockImplementation(({ request }) => {
      interceptor = request

      return vi.fn()
    })

    const signMock = vi
      .fn()
      .mockRejectedValueOnce(new KeysError(KeysErrorCode.NO_KEYS))
      .mockRejectedValueOnce(new KeysError(KeysErrorCode.NO_KEYS))
      .mockResolvedValue(new ArrayBuffer(10))

    vi.mocked(useSignDetached).mockReturnValue(signMock)

    mount(TestComponent)

    expect(fetchInterceptMock.register).toHaveBeenCalledOnce()
    expectToBeDefined(interceptor)

    expect(signMock).not.toHaveBeenCalled()
    const interceptPromise = interceptor('/api', undefined)

    // First error
    expect(signMock).toHaveBeenCalledOnce()

    // Second error
    await vi.advanceTimersByTimeAsync(100)
    expect(signMock).toHaveBeenCalledTimes(2)

    // Keys resolved
    await vi.advanceTimersByTimeAsync(100)
    expect(signMock).toHaveBeenCalledTimes(3)

    const [url, config] = await interceptPromise
    const headers = Array.from(config.headers) as [string, string]

    expect(url).toBe('/api')
    expect(headers).toMatchObject(
      expect.arrayContaining([
        ['grpc-metadata-x-sidero-signature', 'siderov1 testuser public_key_id AAAAAAAAAAAAAA=='],
      ]),
    )
  })
})

function expectToBeDefined<T>(value: T | undefined): asserts value is T {
  expect(value).toBeDefined()
}
