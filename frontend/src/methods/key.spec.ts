// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { server } from '@msw/server'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { add, milliseconds, sub } from 'date-fns'
import { http, HttpResponse } from 'msw'
import {
  generateKey,
  type Key,
  type PartialConfig,
  type PrivateKey,
  readKey,
  readPrivateKey,
  sign,
  type WebStream,
} from 'openpgp/lightweight'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import { defineComponent, nextTick } from 'vue'
import { useRouter } from 'vue-router'

import type { RegisterPublicKeyRequest, RegisterPublicKeyResponse } from '@/api/omni/auth/auth.pb'

import { createKeys, hasValidKeys, useKeys, useSignDetached, useWatchKeyExpiry } from './key'

vi.mock('openpgp', () => ({
  enums: {
    compression: { zlib: 'zlib' },
    symmetric: { aes256: 'aes256' },
    hash: { sha256: 'sha256' },
  },
  sign: vi.fn(),
  createMessage: vi.fn(),
  generateKey: vi.fn(),
  readPrivateKey: vi.fn(),
  readKey: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: vi.fn().mockReturnValue({ fullPath: 'fullPath' }),
  useRouter: vi.fn().mockReturnValue({ replace: vi.fn() }),
}))

describe('useKeys', () => {
  beforeEach(() => {
    // vi.mocked doesn't work with overloads unfortunately
    type ReadPrivateKeyFunc = (options: {
      armoredKey: string
      config?: PartialConfig
    }) => Promise<PrivateKey>
    type ReadKeyFunc = (options: { armoredKey: string; config?: PartialConfig }) => Promise<Key>

    vi.mocked(readPrivateKey as ReadPrivateKeyFunc).mockImplementation(
      async (o) => `private-✨${o.armoredKey}✨` as unknown as PrivateKey,
    )
    vi.mocked(readKey as ReadKeyFunc).mockImplementation(
      async (o) => `public-✨${o.armoredKey}✨` as unknown as PrivateKey,
    )
  })

  afterEach(() => {
    useKeys().clear()

    vi.mocked(readPrivateKey).mockReset()
    vi.mocked(readKey).mockReset()
  })

  test('defaults to null/undefined', () => {
    const { privateKey, privateKeyArmored, publicKey, publicKeyArmored, publicKeyID } = useKeys()

    expect(privateKeyArmored.value).toBeNull()
    expect(privateKey.value).toBeUndefined()
    expect(publicKeyArmored.value).toBeNull()
    expect(publicKey.value).toBeUndefined()
    expect(publicKeyID.value).toBeNull()
  })

  test('sets values', async () => {
    const { privateKey, privateKeyArmored, publicKey, publicKeyArmored, publicKeyID } = useKeys()

    privateKeyArmored.value = '1'
    publicKeyArmored.value = '2'
    publicKeyID.value = '3'

    expect(privateKeyArmored.value).toBe('1')
    await expect(privateKey.value).resolves.toBe('private-✨1✨')
    expect(publicKeyArmored.value).toBe('2')
    await expect(publicKey.value).resolves.toBe('public-✨2✨')
    expect(publicKeyID.value).toBe('3')
  })

  test('clears values', () => {
    const { privateKey, privateKeyArmored, publicKey, publicKeyArmored, publicKeyID, clear } =
      useKeys()

    privateKeyArmored.value = '1'
    publicKeyArmored.value = '2'
    publicKeyID.value = '3'

    clear()

    expect(privateKeyArmored.value).toBeNull()
    expect(privateKey.value).toBeUndefined()
    expect(publicKeyArmored.value).toBeNull()
    expect(publicKey.value).toBeUndefined()
    expect(publicKeyID.value).toBeNull()
  })

  test('persists values', async () => {
    // First pass
    await (async () => {
      const { privateKey, privateKeyArmored, publicKey, publicKeyArmored, publicKeyID } = useKeys()

      privateKeyArmored.value = '1'
      publicKeyArmored.value = '2'
      publicKeyID.value = '3'

      expect(privateKeyArmored.value).toBe('1')
      await expect(privateKey.value).resolves.toBe('private-✨1✨')
      expect(publicKeyArmored.value).toBe('2')
      await expect(publicKey.value).resolves.toBe('public-✨2✨')
      expect(publicKeyID.value).toBe('3')
    })()

    // actual storage is updated on the next tick
    await nextTick()

    // Second pass
    await (async () => {
      const { privateKey, privateKeyArmored, publicKey, publicKeyArmored, publicKeyID } = useKeys()

      expect(privateKeyArmored.value).toBe('1')
      await expect(privateKey.value).resolves.toBe('private-✨1✨')
      expect(publicKeyArmored.value).toBe('2')
      await expect(publicKey.value).resolves.toBe('public-✨2✨')
      expect(publicKeyID.value).toBe('3')
    })()
  })
})

describe('useWatchKeyExpiry', () => {
  const now = 1762881349000
  const TestComponent = defineComponent({
    setup: useWatchKeyExpiry,
    template: '<template />',
  })
  const getExpirationTime = vi.fn()

  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(now)

    vi.mocked(readPrivateKey, { partial: true }).mockResolvedValue({ getExpirationTime })
  })

  enableAutoUnmount(afterEach)

  afterEach(() => {
    useKeys().clear()

    getExpirationTime.mockReset()
    vi.mocked(readPrivateKey).mockReset()
    vi.mocked(useRouter().replace).mockReset()

    vi.useRealTimers()
  })

  test('does nothing if no privateKey', async () => {
    mount(TestComponent)

    await flushPromises()

    expect(getExpirationTime).not.toHaveBeenCalled()
  })

  test('clears keys if invalid expiry', async () => {
    getExpirationTime.mockResolvedValue(Infinity)
    useKeys().privateKeyArmored.value = 'whatever'

    mount(TestComponent)

    await flushPromises()

    expect(getExpirationTime).toHaveBeenCalledExactlyOnceWith()
    expect(useKeys().privateKeyArmored.value).toBeFalsy()
    expect(useRouter().replace).toHaveBeenCalledExactlyOnceWith({
      name: 'Authenticate',
      query: { flow: 'frontend', redirect: 'fullPath' },
    })
  })

  test('clears keys on expiry', async () => {
    getExpirationTime.mockResolvedValue(add(now, { minutes: 1 }))
    useKeys().privateKeyArmored.value = 'whatever'

    mount(TestComponent)

    await flushPromises()

    expect(getExpirationTime).toHaveBeenCalledExactlyOnceWith()
    expect(useKeys().privateKeyArmored.value).toBeDefined()
    expect(useRouter().replace).not.toHaveBeenCalled()

    vi.advanceTimersByTime(milliseconds({ minutes: 2 }))

    expect(useKeys().privateKeyArmored.value).toBeFalsy()
    expect(useRouter().replace).toHaveBeenCalledExactlyOnceWith({
      name: 'Authenticate',
      query: { flow: 'frontend', redirect: 'fullPath' },
    })
  })

  test('cleans up on unmount', async () => {
    getExpirationTime.mockResolvedValue(add(now, { minutes: 1 }))
    useKeys().privateKeyArmored.value = 'whatever'

    const wrapper = mount(TestComponent)

    await flushPromises()

    expect(getExpirationTime).toHaveBeenCalledExactlyOnceWith()
    expect(useKeys().privateKeyArmored.value).toBeDefined()
    expect(useRouter().replace).not.toHaveBeenCalled()

    wrapper.unmount()
    vi.advanceTimersByTime(milliseconds({ minutes: 2 }))

    expect(useKeys().privateKeyArmored.value).toBeDefined()
    expect(useRouter().replace).not.toHaveBeenCalled()
  })
})

describe('useSignDetached', () => {
  beforeEach(() => {
    // vi.mocked doesn't work with overloads unfortunately
    type ReadPrivateKeyFunc = (options: {
      armoredKey: string
      config?: PartialConfig
    }) => Promise<PrivateKey>

    vi.mocked(readPrivateKey as ReadPrivateKeyFunc).mockImplementation(
      async (o) => `private-✨${o.armoredKey}✨` as unknown as PrivateKey,
    )

    vi.mocked(sign).mockResolvedValue(new Uint8Array([1, 2, 3]) as WebStream<Uint8Array>)
  })

  afterEach(() => {
    useKeys().clear()

    vi.mocked(sign).mockReset()
    vi.mocked(readPrivateKey).mockReset()
  })

  test('uses key from params if available', async () => {
    const signDetached = useSignDetached()
    useKeys().privateKeyArmored.value = 'storage-key'

    const signature = await signDetached('data', 'param-key')

    expect(sign).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({
        signingKeys: 'private-✨param-key✨',
      }),
    )

    expect(signature).toBe('AQID')
  })

  test('uses key from useKeys if no param', async () => {
    const signDetached = useSignDetached()
    useKeys().privateKeyArmored.value = 'storage-key'

    const signature = await signDetached('data')

    expect(sign).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({
        signingKeys: `private-✨storage-key✨`,
      }),
    )

    expect(signature).toBe('AQID')
  })

  test('throws an error if no signing key is found', async () => {
    const signDetached = useSignDetached()

    await expect(signDetached('data')).rejects.toThrowError()
  })
})

describe('hasValidKeys', () => {
  const getExpirationTime = vi.fn()

  beforeEach(() => {
    vi.mocked(readPrivateKey, { partial: true }).mockResolvedValue({ getExpirationTime })
  })

  afterEach(() => {
    useKeys().clear()

    getExpirationTime.mockReset()
    vi.mocked(readPrivateKey).mockReset()
  })

  test('false if no privateKey', async () => {
    await expect(hasValidKeys()).resolves.toBeFalsy()
  })

  test('false if no expiration time', async () => {
    useKeys().privateKeyArmored.value = 'whatever'
    getExpirationTime.mockResolvedValue(Infinity)

    await expect(hasValidKeys()).resolves.toBeFalsy()
  })

  test('false if expired', async () => {
    useKeys().privateKeyArmored.value = 'whatever'
    getExpirationTime.mockResolvedValue(sub(Date.now(), { seconds: 10 }))

    await expect(hasValidKeys()).resolves.toBeFalsy()
  })

  test('true if has valid expiration in the future', async () => {
    useKeys().privateKeyArmored.value = 'whatever'
    getExpirationTime.mockResolvedValue(add(Date.now(), { seconds: 10 }))

    await expect(hasValidKeys()).resolves.toBeTruthy()
  })
})

describe('createKeys', () => {
  test('creates & registers keys with the api', async () => {
    vi.mocked(generateKey, { partial: true }).mockResolvedValue({
      privateKey: 'privateKey' as unknown as PrivateKey,
      publicKey: 'publicKey' as unknown as Key,
    })

    server.use(
      http.post<never, RegisterPublicKeyRequest>('/auth.AuthService/RegisterPublicKey', () => {
        return HttpResponse.json<RegisterPublicKeyResponse>(
          { public_key_id: 'public_key_id' },
          {
            headers: {
              'content-type': 'application/json',
              'Grpc-metadata-content-type': 'application/grpc',
            },
          },
        )
      }),
    )

    const { privateKey, publicKey, publicKeyId } = await createKeys('myFANCY@email.COM')

    expect(generateKey).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({
        userIDs: [{ email: 'myfancy@email.com' }],
      }),
    )

    expect(privateKey).toBe('privateKey')
    expect(publicKey).toBe('publicKey')
    expect(publicKeyId).toBe('public_key_id')
  })
})
