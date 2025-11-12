// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useLocalStorage } from '@vueuse/core'
import { differenceInMilliseconds, isAfter, milliseconds, millisecondsToSeconds } from 'date-fns'
import {
  createMessage,
  enums,
  generateKey,
  readKey,
  readPrivateKey,
  sign,
} from 'openpgp/lightweight'
import { computed, watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { b64Encode } from '@/api/fetch.pb'
import { AuthService } from '@/api/omni/auth/auth.pb'
import { AuthFlowQueryParam, FrontendAuthFlow, RedirectQueryParam } from '@/api/resources'

const privateKeyArmored = useLocalStorage<string>('privateKey', null)
const publicKeyArmored = useLocalStorage<string>('publicKey', null)
const publicKeyID = useLocalStorage<string>('publicKeyID', null)

const privateKey = computed(() => {
  if (!privateKeyArmored.value) return

  return readPrivateKey({ armoredKey: privateKeyArmored.value })
})

const publicKey = computed(() => {
  if (!publicKeyArmored.value) return

  return readKey({ armoredKey: publicKeyArmored.value })
})

export function useKeys() {
  return {
    privateKey,
    privateKeyArmored,
    publicKey,
    publicKeyArmored,
    publicKeyID,
    clear() {
      privateKeyArmored.value = null
      publicKeyArmored.value = null
      publicKeyID.value = null
    },
  }
}

/**
 * We register this hook to watch the state of our keys
 * and to automatically send the user to the authentication page
 * when keys are expire.
 */
export function useWatchKeyExpiry() {
  const router = useRouter()
  const route = useRoute()
  const keys = useKeys()

  watchEffect(async (onCleanup) => {
    if (!keys.privateKey.value) return

    const key = await keys.privateKey.value
    const keyExpirationTime = await key.getExpirationTime()

    if (!(keyExpirationTime instanceof Date) || isAfter(Date.now(), keyExpirationTime)) {
      return clearKeysAndRedirect()
    }

    const keysReloadTimeout = window.setTimeout(
      clearKeysAndRedirect,
      differenceInMilliseconds(keyExpirationTime, Date.now()) + 1000,
    )

    onCleanup(() => clearTimeout(keysReloadTimeout))

    function clearKeysAndRedirect() {
      keys.clear()

      router.replace({
        name: 'Authenticate',
        query: { [AuthFlowQueryParam]: FrontendAuthFlow, [RedirectQueryParam]: route.fullPath },
      })
    }
  })
}

export function useSignDetached() {
  const keys = useKeys()

  return async function (data: string, privateKey?: string) {
    const signingKeys = privateKey
      ? await readPrivateKey({ armoredKey: privateKey })
      : await keys.privateKey.value

    if (!signingKeys) {
      throw new Error('failed to load keys: keys not initialized')
    }

    const stream = await sign({
      message: await createMessage({ text: data }),
      detached: true,
      signingKeys,
      format: 'binary',
    })

    const array = stream as Uint8Array

    return b64Encode(array, 0, array.length)
  }
}

export async function hasValidKeys() {
  if (!privateKey.value) return false

  const key = await privateKey.value
  const keyExpirationTime = await key.getExpirationTime()

  if (!keyExpirationTime) return false

  return isAfter(keyExpirationTime, Date.now())
}

export async function createKeys(email: string) {
  email = email.toLowerCase()

  const { privateKey, publicKey } = await generateKey({
    type: 'ecc',
    curve: 'ed25519Legacy',
    userIDs: [{ email }],
    keyExpirationTime: millisecondsToSeconds(milliseconds({ hours: 7, minutes: 50 })),
    config: {
      preferredCompressionAlgorithm: enums.compression.zlib,
      preferredSymmetricAlgorithm: enums.symmetric.aes256,
      preferredHashAlgorithm: enums.hash.sha256,
    },
  })

  const enc = new TextEncoder()

  const response = await AuthService.RegisterPublicKey({
    public_key: {
      pgp_data: enc.encode(publicKey),
    },
    identity: { email },
  })

  return {
    privateKey,
    publicKey,
    publicKeyId: response.public_key_id!,
  }
}
