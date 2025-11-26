// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { getUnixTime } from 'date-fns'
import fetchIntercept, { type FetchInterceptor } from 'fetch-intercept'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import { defineComponent, ref } from 'vue'

import { signDetached, useKeys } from '@/methods/key'

import { useRegisterAPIInterceptor } from './interceptor'

vi.mock('fetch-intercept')
vi.mock('@/methods/identity', () => ({
  useIdentity: vi.fn(() => ({ identity: ref('testuser') })),
}))
vi.mock(import('@/methods/key'), async (importOriginal) => {
  const original = await importOriginal()
  return {
    ...original,
    useKeys: vi.fn(() => ({
      keyPair: ref(mockKey),
      publicKeyID: ref('public_key_id'),
      keyExpirationTime: ref(null),
      clear: vi.fn(),
    })),
    signDetached: vi.fn().mockResolvedValue(new ArrayBuffer(10)),
  }
})

const fetchInterceptMock = vi.mocked(fetchIntercept)
const mockKey = {
  privateKey: await crypto.subtle.importKey(
    'jwk',
    {
      key_ops: ['sign'],
      ext: true,
      kty: 'EC',
      x: '783KrEU9o1ZPATh2FZFiaJUOct3IiVt1GAQ6eNx-iHc',
      y: 'LPg7JSWJePeCGNWvzoTbhuhDV5AFk7RTAq5HYA2CgdY',
      crv: 'P-256',
      d: 'o_RIDjrl21hFPaTiyXmPBq_b5EsWw9p_bel_-Bwi45g',
    },
    { name: 'ECDSA', namedCurve: 'P-256' },
    true,
    ['sign'],
  ),
  publicKey: await crypto.subtle.importKey(
    'jwk',
    {
      key_ops: ['verify'],
      ext: true,
      kty: 'EC',
      x: '783KrEU9o1ZPATh2FZFiaJUOct3IiVt1GAQ6eNx-iHc',
      y: 'LPg7JSWJePeCGNWvzoTbhuhDV5AFk7RTAq5HYA2CgdY',
      crv: 'P-256',
    },
    { name: 'ECDSA', namedCurve: 'P-256' },
    true,
    ['verify'],
  ),
}

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

    const keyPairRef = ref<CryptoKeyPair | null>(null)

    vi.mocked(useKeys).mockReturnValue({
      keyPair: keyPairRef,
      publicKeyID: ref('public_key_id'),
      keyExpirationTime: ref(null),
      clear: vi.fn(),
    })

    mount(TestComponent)

    expect(fetchInterceptMock.register).toHaveBeenCalledOnce()
    expectToBeDefined(interceptor)

    expect(signDetached).not.toHaveBeenCalled()
    const interceptPromise = interceptor('/api', undefined)

    await vi.runAllTimersAsync()
    expect(signDetached).not.toHaveBeenCalled()

    // Simulate keys becoming available
    keyPairRef.value = mockKey
    await vi.runAllTimersAsync()

    expect(signDetached).toHaveBeenCalledOnce()

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
