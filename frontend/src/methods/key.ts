// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { useLocalStorage } from '@vueuse/core'
import * as fetchIntercept from 'fetch-intercept'
import type { Key, PrivateKey } from 'openpgp/lightweight'
import {
  createMessage,
  enums,
  generateKey,
  readKey,
  readPrivateKey,
  sign,
} from 'openpgp/lightweight'
import { ref } from 'vue'

import { b64Encode } from '@/api/fetch.pb'
import { AuthService } from '@/api/omni/auth/auth.pb'
import {
  authHeader,
  PayloadHeaderKey,
  SignatureHeaderKey,
  SignatureVersionV1,
  TimestampHeaderKey,
} from '@/api/resources'

let interceptorsRegistered = false
let keysReloadTimeout: number

const privateKeyArmored = useLocalStorage<string>('privateKey', null)
const publicKeyArmored = useLocalStorage<string>('publicKey', null)
const publicKeyID = useLocalStorage<string>('publicKeyID', null)

export const identity = useLocalStorage<string>('identity', null)
export const fullname = useLocalStorage<string>('fullname', null)
export const avatar = useLocalStorage<string>('avatar', null)

export let keys: {
  privateKey: PrivateKey
  publicKey: Key
  identity: string
} | null

export class KeysInvalidError extends Error {
  constructor(msg: string) {
    super(msg)

    // Set the prototype explicitly.
    Object.setPrototypeOf(this, KeysInvalidError.prototype)
  }
}

export const authorized = ref(false)

export const isAuthorized = async (): Promise<boolean> => {
  if (!keys) {
    try {
      await loadKeys()
    } catch {
      return false
    }
  }

  const keyExpirationTime = await keys?.privateKey.getExpirationTime()

  if (!keyExpirationTime || new Date() > keyExpirationTime) {
    return false
  }

  return true
}

const loadKeys = async (): Promise<{ privateKey: PrivateKey; publicKey: Key }> => {
  if (!keys) {
    if (!privateKeyArmored.value || !publicKeyArmored.value || !identity.value) {
      throw new KeysInvalidError(`failed to load keys: keys not initialized`)
    }

    keys = {
      privateKey: await readPrivateKey({ armoredKey: privateKeyArmored.value }),
      publicKey: await readKey({ armoredKey: publicKeyArmored.value }),
      identity: identity.value.toLowerCase(),
    }
  }

  const now = new Date()

  const keyExpirationTime = await keys.privateKey.getExpirationTime()

  if (!keyExpirationTime || now > keyExpirationTime) {
    throw new KeysInvalidError(`failed to load keys: the key is expired`)
  }

  registerInterceptors()

  clearTimeout(keysReloadTimeout)
  keysReloadTimeout = window.setTimeout(
    () => {
      location.reload()
    },
    (keyExpirationTime as Date).getTime() - now.getTime() + 1000,
  )

  authorized.value = true

  return keys
}

export const createKeys = async (
  email: string,
): Promise<{ privateKey: string; publicKey: string; publicKeyId: string }> => {
  email = email.toLowerCase()

  const res = await genKey(email)

  const enc = new TextEncoder()

  const response = await AuthService.RegisterPublicKey({
    public_key: {
      pgp_data: enc.encode(res.publicKey),
    },
    identity: {
      email: email,
    },
  })

  return {
    publicKeyId: response.public_key_id!,
    ...res,
  }
}

export const revokePublicKey = async () => {
  if (!publicKeyID.value) {
    return
  }

  await AuthService.RevokePublicKey({
    public_key_id: publicKeyID.value,
  })
}

export const signDetached = async (data: string, privateKey?: string): Promise<string> => {
  let keys: {
    privateKey: PrivateKey
  }

  if (privateKey) {
    keys = {
      privateKey: await readPrivateKey({ armoredKey: privateKey }),
    }
  } else {
    keys = await loadKeys()
  }

  const stream = await sign({
    message: await createMessage({ text: data }),
    detached: true,
    signingKeys: keys.privateKey,
    format: 'binary',
  })

  const array = stream as Uint8Array

  return b64Encode(array, 0, array.length)
}

export const saveKeys = async (
  user: { email: string; picture: string; fullname: string },
  privateKey: string,
  publicKey: string,
  publicKeyId: string,
) => {
  keys = null

  publicKeyArmored.value = publicKey
  privateKeyArmored.value = privateKey
  publicKeyID.value = publicKeyId

  identity.value = user.email.toLowerCase()
  avatar.value = user.picture
  fullname.value = user.fullname

  const loadedKeys = await loadKeys()

  const expirationTime = await loadedKeys.privateKey.getExpirationTime()

  if (!(expirationTime instanceof Date)) {
    throw new KeysInvalidError('failed to save keys: invalid expiration time')
  }
}

export const resetKeys = () => {
  keys = null

  publicKeyArmored.value = undefined
  privateKeyArmored.value = undefined
  publicKeyID.value = undefined

  identity.value = undefined
  avatar.value = undefined
  fullname.value = undefined

  authorized.value = false
}

const genKey = async (email: string): Promise<{ publicKey: string; privateKey: string }> => {
  const { privateKey, publicKey } = await generateKey({
    type: 'ecc',
    curve: 'ed25519Legacy',
    userIDs: [{ email: email.toLowerCase() }],
    keyExpirationTime: 7 * 60 * 60 + 50 * 60, // 7 hours 50 minutes
    config: {
      preferredCompressionAlgorithm: enums.compression.zlib,
      preferredSymmetricAlgorithm: enums.symmetric.aes256,
      preferredHashAlgorithm: enums.hash.sha256,
    },
  })

  return {
    publicKey: publicKey,
    privateKey: privateKey,
  }
}

const includedHeaders = [
  'nodes',
  'selectors',
  'fieldSelectors',
  'runtime',
  'context',
  'cluster',
  'namespace',
  'uid',
  TimestampHeaderKey,
  authHeader,
]

const buildPayload = (
  url: string,
  config: { headers?: Headers },
): { headers: Record<string, string[]>; method: string } => {
  const headers: Record<string, string[]> = {}

  if (config.headers) {
    for (const header of includedHeaders) {
      const key = `Grpc-Metadata-${header}`
      const value = config.headers.get(key)

      if (value) {
        if (!headers[header]) {
          headers[header] = [value]
        } else {
          headers[header].push(value)
        }
      }
    }
  }

  return {
    headers: headers,
    method: url.replace(/^\/api/, ''),
  }
}

const registerInterceptors = () => {
  if (interceptorsRegistered) {
    return
  }

  fetchIntercept.register({
    request: async (url, config?: { headers?: Headers; method?: string }) => {
      url = encodeURI(url)

      if (
        !/^\/(api|image)/.test(url) ||
        (url.startsWith('/api/auth.') && !url.startsWith('/api/auth.AuthService/RevokePublicKey'))
      ) {
        return [url, config]
      }

      if (!config) {
        config = {}
      }

      if (!config.headers) {
        config.headers = new Headers()
      }

      const ts = (new Date().getTime() / 1000).toFixed(0)

      try {
        if (url.indexOf('/api') === 0) {
          config.headers.set(`Grpc-Metadata-${TimestampHeaderKey}`, ts)

          const payload = JSON.stringify(buildPayload(url, config))
          const signature = await signDetached(payload)
          const fingerprint = keys?.publicKey.getFingerprint()

          config.headers.set(`Grpc-Metadata-${PayloadHeaderKey}`, payload)
          config.headers.set(
            `Grpc-Metadata-${SignatureHeaderKey}`,
            `${SignatureVersionV1} ${keys?.identity} ${fingerprint} ${signature}`,
          )
        } else if (url.indexOf('/image/') === 0) {
          config.headers.set(TimestampHeaderKey, ts)

          const sha256 = 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855' // empty string sha256
          const payload = [config.method ?? 'GET', url, ts, sha256].join('\n')
          const signature = await signDetached(payload)
          const fingerprint = keys?.publicKey.getFingerprint()

          config.headers.set(
            SignatureHeaderKey,
            `${SignatureVersionV1} ${keys?.identity} ${fingerprint} ${signature}`,
          )
        }
      } catch {
        // reload the page to make the key Authenticator regenerate the key
        location.reload()
      }

      return [url, config]
    },
  })

  interceptorsRegistered = true
}

export const getParentDomain = () => {
  const domainParts = window.location.hostname.split('.')
  if (domainParts.length < 2) {
    console.error(
      'there is no parent domain for the current hostname, returning the current hostname',
    )

    return window.location.hostname
  }

  return domainParts.slice(1).join('.')
}
