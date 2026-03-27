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
      } else if (url.startsWith('/image')) {
        config.headers.set(TimestampHeaderKey, ts)

        const sha256 = 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855' // empty string sha256
        const payload = [config.method ?? 'GET', url, ts, sha256].join('\n')
        const signature = await generateSignatureHeader(payload)

        config.headers.set(SignatureHeaderKey, signature)
      }

      return [url, config]
    },

    response(response) {
      if (
        response.status === 401 &&
        keyPair.value &&
        isSignedRequest(new URL(response.url).pathname)
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

    const array = new Uint8Array(await signDetached(payload, keyPair.value!))
    const signature = b64Encode(array, 0, array.length)
    const fingerprint = publicKeyID.value

    return `${SignatureVersionV1} ${identity.value} ${fingerprint} ${signature}`
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

function isSignedRequest(path: string): boolean {
  return (
    /^\/(api|image)/.test(path) &&
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
