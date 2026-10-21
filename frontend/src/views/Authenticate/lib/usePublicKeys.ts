// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { type MaybeRefOrGetter, nextTick, ref, toValue } from 'vue'
import { useRoute } from 'vue-router'

import { b64Encode, RequestError } from '@/api/fetch.pb'
import { Code } from '@/api/google/rpc/code.pb'
import { AuthService } from '@/api/omni/auth/auth.pb'
import { withMetadata } from '@/api/options'
import { workloadProxyPublicKeyIdSignatureBase64Cookie } from '@/api/resources'
import { useIdentity } from '@/methods/identity'
import { createKeys, signDetached, useKeys } from '@/methods/key'
import type { AuthProvider } from '@/methods/useAuthProvider'
import { showError } from '@/notification'

const retriedQueryParam = 'retried'

/**
 * useConfirmPublicKey confirms a public key with the identity provider's credentials.
 *
 * If Omni rejects the credentials, the user is sent through the login once more,
 * and a second rejection shows the error, so a rejected token cannot cause a login loop.
 */
export function useConfirmPublicKey(provider: MaybeRefOrGetter<AuthProvider>) {
  const route = useRoute()
  const confirming = ref(false)
  const confirmed = ref(false)

  async function confirmPublicKey(publicKeyId: string, keyPair?: CryptoKeyPair) {
    if (confirming.value) return false

    confirming.value = true

    try {
      const metadata = toValue(provider).credentials()

      if (keyPair) {
        const array = new Uint8Array(await signDetached(publicKeyId, keyPair))

        metadata[workloadProxyPublicKeyIdSignatureBase64Cookie] = b64Encode(array, 0, array.length)
      }

      await AuthService.ConfirmPublicKey({ public_key_id: publicKeyId }, withMetadata(metadata))

      confirmed.value = true
    } catch (e) {
      if (
        e instanceof RequestError &&
        e.code === Code.UNAUTHENTICATED &&
        toValue(provider).canRetryLogin
      ) {
        if (route.query[retriedQueryParam]) {
          showError('Failed to confirm public key, please start the login again', e.message)
        } else {
          const query = new URLSearchParams(window.location.search)

          query.set(retriedQueryParam, 'true')

          await toValue(provider).login(query, true)
        }
      } else {
        showError('Failed to confirm public key', e instanceof Error ? e.message : String(e))
      }
    } finally {
      confirming.value = false
    }

    return confirmed.value
  }

  return { confirmPublicKey, confirming, confirmed }
}

/**
 * useIssueBrowserKeys creates a key pair for this browser, confirms it and stores it for the frontend to use.
 */
export function useIssueBrowserKeys(provider: MaybeRefOrGetter<AuthProvider>) {
  const keys = useKeys()
  const identityStorage = useIdentity()
  const { confirmPublicKey, confirmed } = useConfirmPublicKey(provider)
  const issuing = ref(false)

  async function issueKeys() {
    const user = toValue(provider).user.value

    if (issuing.value || !user) return false

    if (!user.email) {
      showError('Failed to generate keys', 'An email is required to issue keys')

      return false
    }

    issuing.value = true

    try {
      let res: Awaited<ReturnType<typeof createKeys>>

      try {
        res = await createKeys(user.email)
      } catch (e) {
        showError('Failed to generate keys', e instanceof Error ? e.message : String(e))

        return false
      }

      if (!(await confirmPublicKey(res.publicKeyId, res.keyPair))) return false

      identityStorage.identity.value = user.email.toLowerCase()
      identityStorage.fullname.value = user.name ?? ''
      identityStorage.avatar.value = user.picture ?? ''

      keys.keyExpirationTime.value = res.keyExpirationTime
      keys.publicKeyID.value = res.publicKeyId
      await keys.setKeyPair(res.keyPair)

      // Wait for storages to be set
      await nextTick()

      return true
    } finally {
      issuing.value = false
    }
  }

  return { issueKeys, issuing, confirmed }
}
