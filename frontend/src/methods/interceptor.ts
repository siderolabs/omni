// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { until } from '@vueuse/core'
import { getUnixTime } from 'date-fns'
import fetchIntercept from 'fetch-intercept'
import { onScopeDispose } from 'vue'

import { b64Encode, type RequestOptions } from '@/api/fetch.pb'
import {
  authHeader,
  PayloadHeaderKey,
  SignatureHeaderKey,
  SignatureVersionV1,
  TimestampHeaderKey,
} from '@/api/resources'
import { useIdentity } from '@/methods/identity'
import { signDetached, useKeys } from '@/methods/key'

export interface OmniRequestOptions extends RequestOptions {
  skipSignature?: boolean
}

/**
 * useRegisterAPIInterceptor registers an interceptor on the global fetch.
 * This will add the necessary authorization headers for Omni gRPC calls.
 */
export function useRegisterAPIInterceptor() {
  const { keyPair, publicKeyID, invalidate: invalidateKeys } = useKeys()
  const { identity } = useIdentity()

  const unregister = fetchIntercept.register({
    async request(url, config?: RequestInit) {
      url = encodeURI(url)

      if (!isSignedRequest(url) || (config as OmniRequestOptions)?.skipSignature) {
        return [url, config]
      }

      config ||= {}

      if (!(config.headers instanceof Headers)) {
        config.headers = new Headers(config.headers)
      }

      const ts = getUnixTime(Date.now()).toString()

      if (url.startsWith('/api')) {
        config.headers.set(`Grpc-Metadata-${TimestampHeaderKey}`, ts)

        const payload = JSON.stringify(buildPayload(url, config.headers))
        const signature = await generateSignatureHeader(payload)

        config.headers.set(`Grpc-Metadata-${PayloadHeaderKey}`, payload)
        config.headers.set(`Grpc-Metadata-${SignatureHeaderKey}`, signature)
      }

      return [url, config]
    },

    response(response) {
      // Only a rejection of the current key invalidates it. A request signed with a key
      // that was replaced while it was in flight, e.g. by a login in another tab, is not.
      if (
        response.status === 401 &&
        keyPair.value &&
        isSignedRequest(new URL(response.url).pathname) &&
        signingKeyID(response.request) === publicKeyID.value
      ) {
        invalidateKeys()
      }

      return response
    },
  })

  onScopeDispose(unregister)

  async function generateSignatureHeader(payload: string) {
    if (!keyPair.value) {
      // Wait for keys to be created.
      await until(keyPair).toBeTruthy()
    }

    // Read the key together with its ID and identity, as they may be replaced while signing
    const key = keyPair.value!
    const fingerprint = publicKeyID.value
    const email = identity.value

    const array = new Uint8Array(await signDetached(payload, key))
    const signature = b64Encode(array, 0, array.length)

    return `${SignatureVersionV1} ${email} ${fingerprint} ${signature}`
  }
}

/** Returns the ID of the public key that signed the request, see generateSignatureHeader. */
function signingKeyID(request?: Request) {
  return request?.headers.get(`Grpc-Metadata-${SignatureHeaderKey}`)?.split(' ')[2]
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

function isSignedRequest(path: string): boolean {
  return (
    /^\/api/.test(path) &&
    (!path.startsWith('/api/auth.') || path.startsWith('/api/auth.AuthService/RevokePublicKey'))
  )
}

function buildPayload(url: string, reqHeaders: Headers) {
  const headers: Record<string, string[]> = {}

  if (reqHeaders) {
    for (const header of includedHeaders) {
      const key = `Grpc-Metadata-${header}`
      const value = reqHeaders.get(key)

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
    headers,
    method: url.replace(/^\/api/, ''),
  }
}
