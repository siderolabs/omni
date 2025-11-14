// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useLocalStorage } from '@vueuse/core'
import { useIDBKeyval } from '@vueuse/integrations/useIDBKeyval'
import { add, differenceInMilliseconds, formatRFC3339, isAfter } from 'date-fns'
import { watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { b64Encode } from '@/api/fetch.pb'
import { AuthService } from '@/api/omni/auth/auth.pb'
import { AuthFlowQueryParam, FrontendAuthFlow, RedirectQueryParam } from '@/api/resources'

const { data: keyPair, isFinished: keyPairLoaded } = useIDBKeyval<CryptoKeyPair | null>(
  'keyPair',
  null,
)
const keyExpirationTime = useLocalStorage<Date | null>('keyExpirationTime', null)
const publicKeyID = useLocalStorage<string | null>('publicKeyID', null)

export function useKeys() {
  return {
    keyPair,
    keyExpirationTime,
    publicKeyID,
    clear() {
      keyPair.value = null
      keyExpirationTime.value = null
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
    if (!keys.keyPair.value) return

    if (!keys.keyExpirationTime.value || isAfter(Date.now(), keys.keyExpirationTime.value)) {
      return clearKeysAndRedirect()
    }

    const keysReloadTimeout = window.setTimeout(
      clearKeysAndRedirect,
      differenceInMilliseconds(keys.keyExpirationTime.value, Date.now()) + 1000,
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

  return async function (data: string, keyPair = keys.keyPair.value) {
    if (!keyPair) {
      throw new Error('failed to load keys: keys not initialized')
    }

    return await crypto.subtle.sign(
      { name: 'ECDSA', hash: 'SHA-256' },
      keyPair.privateKey,
      new TextEncoder().encode(data),
    )
  }
}

export async function hasValidKeys() {
  // IndexedDB is async storage, and might not yet have been initialised
  if (!keyPairLoaded.value) return new Promise((r) => setTimeout(() => r(hasValidKeys()), 20))

  if (!keyPair.value || !keyExpirationTime.value) return false

  return isAfter(keyExpirationTime.value, Date.now())
}

export async function createKeys(email: string) {
  email = email.toLowerCase()

  const keyPair = await crypto.subtle.generateKey({ name: 'ECDSA', namedCurve: 'P-256' }, false, [
    'sign',
    'verify',
  ])

  const buffer = await crypto.subtle.exportKey('spki', keyPair.publicKey)
  const array = new Uint8Array(buffer)
  const publicKeyB64 = b64Encode(array, 0, array.length)
    .match(/.{1,64}/g)
    ?.join('\n')

  const keyExpirationTime = add(new Date(), { hours: 7, minutes: 50 })

  const response = await AuthService.RegisterPublicKey({
    public_key: {
      plain_key: {
        key_pem: `-----BEGIN PUBLIC KEY-----\n${publicKeyB64}\n-----END PUBLIC KEY-----`,
        not_before: formatRFC3339(new Date()),
        not_after: formatRFC3339(keyExpirationTime),
      },
    },
    identity: { email },
  })

  return {
    keyPair,
    keyExpirationTime,
    publicKeyId: response.public_key_id!,
  }
}
