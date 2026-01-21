// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { server } from '@msw/server'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { add, isAfter, isBefore, milliseconds, sub } from 'date-fns'
import { http, HttpResponse } from 'msw'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import { defineComponent, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import type { RegisterPublicKeyRequest, RegisterPublicKeyResponse } from '@/api/omni/auth/auth.pb'

import { createKeys, hasValidKeys, signDetached, useKeys, useWatchKeyExpiry } from './key'

vi.mock('vue-router')

beforeEach(() => {
  useKeys().clear()

  vi.mocked(useRoute, { partial: true }).mockReturnValue({})
  vi.mocked(useRouter, { partial: true }).mockReturnValue({
    isReady: vi.fn(),
    replace: vi.fn(),
  })

  vi.mocked(useRouter().isReady).mockImplementation(async () => {
    vi.mocked(useRoute()).fullPath = 'fullPath'
  })
})

// Exported mock key for testing purposes only
const mockKey: CryptoKeyPair = {
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

describe('useKeys', () => {
  test('defaults to empty', () => {
    const { keyPair, keyExpirationTime, publicKeyID } = useKeys()

    expect(keyPair.value).toBeFalsy()
    expect(keyExpirationTime.value).toBeFalsy()
    expect(publicKeyID.value).toBeFalsy()
  })

  test('clears values', () => {
    const { keyPair, keyExpirationTime, publicKeyID, clear } = useKeys()

    keyPair.value = mockKey
    keyExpirationTime.value = new Date()
    publicKeyID.value = '1'

    clear()

    expect(keyPair.value).toBeFalsy()
    expect(keyExpirationTime.value).toBeFalsy()
    expect(publicKeyID.value).toBeFalsy()
  })

  test('persists values', async () => {
    const date = new Date()

    await (async () => {
      const { keyPair, keyExpirationTime, publicKeyID } = useKeys()

      keyPair.value = mockKey
      keyExpirationTime.value = date
      publicKeyID.value = '1'
    })()

    await (async () => {
      const { keyPair, keyExpirationTime, publicKeyID } = useKeys()

      const actualKey = await crypto.subtle.exportKey('raw', keyPair.value!.publicKey)
      const expectedKey = await crypto.subtle.exportKey('raw', mockKey.publicKey)

      expect(actualKey).toStrictEqual(expectedKey)
      expect(keyExpirationTime.value).toBe(date)
      expect(publicKeyID.value).toBe('1')
    })()
  })
})

describe('useWatchKeyExpiry', () => {
  const now = 1762881349000
  const TestComponent = defineComponent({
    setup: useWatchKeyExpiry,
    template: '<template />',
  })

  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(now)
  })

  enableAutoUnmount(afterEach)

  afterEach(() => {
    vi.useRealTimers()
  })

  test('does nothing if no keyPair', async () => {
    mount(TestComponent)

    await nextTick()

    expect(useRouter().replace).not.toHaveBeenCalled()
  })

  test('clears keys if invalid expiry', async () => {
    useKeys().keyPair.value = mockKey

    mount(TestComponent)

    await nextTick()

    expect(useKeys().keyPair.value).toBeFalsy()
    expect(useRouter().replace).toHaveBeenCalledExactlyOnceWith({
      name: 'Authenticate',
      query: { flow: 'frontend', redirect: 'fullPath' },
    })
  })

  test('clears keys on expiry', async () => {
    useKeys().keyExpirationTime.value = add(now, { minutes: 1 })
    useKeys().keyPair.value = mockKey

    mount(TestComponent)

    await nextTick()

    expect(useKeys().keyPair.value).toBeDefined()
    expect(useRouter().replace).not.toHaveBeenCalled()

    vi.advanceTimersByTime(milliseconds({ minutes: 2 }))

    expect(useRouter().replace).not.toHaveBeenCalled()

    await nextTick()

    expect(useKeys().keyPair.value).toBeFalsy()
    expect(useRouter().replace).toHaveBeenCalledExactlyOnceWith({
      name: 'Authenticate',
      query: { flow: 'frontend', redirect: 'fullPath' },
    })
  })

  test('keeps existing auth flow on expiry', async () => {
    useKeys().keyExpirationTime.value = add(now, { minutes: 1 })
    useKeys().keyPair.value = mockKey

    vi.mocked(useRouter().isReady).mockImplementation(async () => {
      vi.mocked(useRoute()).fullPath = 'fullPath'
      vi.mocked(useRoute()).name = 'Authenticate'
    })

    mount(TestComponent)

    await nextTick()

    expect(useKeys().keyPair.value).toBeDefined()
    expect(useRouter().replace).not.toHaveBeenCalled()

    vi.advanceTimersByTime(milliseconds({ minutes: 2 }))

    await nextTick()

    expect(useKeys().keyPair.value).toBeFalsy()
    expect(useRouter().replace).not.toHaveBeenCalled()
  })

  test('cleans up on unmount', async () => {
    useKeys().keyExpirationTime.value = add(now, { minutes: 1 })
    useKeys().keyPair.value = mockKey

    const wrapper = mount(TestComponent)

    await nextTick()

    expect(useKeys().keyPair.value).toBeDefined()
    expect(useRouter().replace).not.toHaveBeenCalled()

    wrapper.unmount()
    vi.advanceTimersByTime(milliseconds({ minutes: 2 }))

    await nextTick()

    expect(useKeys().keyPair.value).toBeDefined()
    expect(useRouter().replace).not.toHaveBeenCalled()
  })
})

describe('signDetached', () => {
  test('signs data', async () => {
    const signature = await signDetached('data', mockKey)

    await expect(
      crypto.subtle.verify(
        { name: 'ECDSA', hash: 'SHA-256' },
        mockKey.publicKey,
        signature,
        new TextEncoder().encode('data'),
      ),
    ).resolves.toBeTruthy()
  })
})

describe('hasValidKeys', () => {
  test('false if no keyPair or expiration time', async () => {
    await expect(hasValidKeys()).resolves.toBeFalsy()
  })

  test('false if expired', async () => {
    useKeys().keyPair.value = mockKey
    useKeys().keyExpirationTime.value = sub(Date.now(), { seconds: 10 })

    await expect(hasValidKeys()).resolves.toBeFalsy()
  })

  test('true if has valid expiration in the future', async () => {
    useKeys().keyPair.value = mockKey
    useKeys().keyExpirationTime.value = add(Date.now(), { seconds: 10 })

    await expect(hasValidKeys()).resolves.toBeTruthy()
  })
})

describe('createKeys', () => {
  test('creates & registers keys with the api', async () => {
    const emailRef = { email: '' }

    server.use(
      http.post<never, RegisterPublicKeyRequest>(
        '/auth.AuthService/RegisterPublicKey',
        async ({ request }) => {
          const body = await request.json()

          emailRef.email = body.identity?.email ?? ''

          return HttpResponse.json<RegisterPublicKeyResponse>(
            { public_key_id: 'public_key_id' },
            {
              headers: {
                'content-type': 'application/json',
                'Grpc-metadata-content-type': 'application/grpc',
              },
            },
          )
        },
      ),
    )

    const {
      keyPair: { privateKey, publicKey },
      keyExpirationTime,
      publicKeyId,
    } = await createKeys('myFANCY@email.COM')

    expect(emailRef.email).toBe('myfancy@email.com')

    expect(privateKey).toBeInstanceOf(CryptoKey)
    expect(publicKey).toBeInstanceOf(CryptoKey)

    await expect(crypto.subtle.exportKey('jwk', privateKey)).rejects.toThrowError(
      'key is not extractable',
    )

    expect(isAfter(keyExpirationTime, sub(Date.now(), { seconds: 5 }))).toBeTruthy()
    expect(isBefore(keyExpirationTime, add(Date.now(), { hours: 8 }))).toBeTruthy()
    expect(publicKeyId).toBe('public_key_id')
  })
})
