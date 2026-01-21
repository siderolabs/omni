// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { getUnixTime } from 'date-fns'
import fetchIntercept from 'fetch-intercept'
import { onBeforeMount, onUnmounted, ref, watch } from 'vue'

import { b64Encode } from '@/api/fetch.pb'
import {
  authHeader,
  PayloadHeaderKey,
  SignatureHeaderKey,
  SignatureVersionV1,
  TimestampHeaderKey,
} from '@/api/resources'
import { useIdentity } from '@/methods/identity'
import { signDetached, useKeys } from '@/methods/key'

/**
 * useRegisterAPIInterceptor registers an interceptor on the global fetch.
 * This will add the necessary authorization headers for Omni gRPC calls.
 */
export function useRegisterAPIInterceptor() {
  const { keyPair, publicKeyID } = useKeys()
  const { identity } = useIdentity()

  const unregisterInterceptor = ref<() => void>()

  onBeforeMount(() => {
    unregisterInterceptor.value = fetchIntercept.register({
      async request(url, config?: { headers?: Headers; method?: string }) {
        url = encodeURI(url)

        if (
          !/^\/(api|image)/.test(url) ||
          (url.startsWith('/api/auth.') && !url.startsWith('/api/auth.AuthService/RevokePublicKey'))
        ) {
          return [url, config]
        }

        config ||= {}
        config.headers ||= new Headers()

        const ts = getUnixTime(Date.now()).toString()

        if (url.startsWith('/api')) {
          config.headers.set(`Grpc-Metadata-${TimestampHeaderKey}`, ts)

          const payload = JSON.stringify(buildPayload(url, config))
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
    })
  })

  async function generateSignatureHeader(payload: string) {
    if (!keyPair.value) {
      // Wait for keys to be created.
      await new Promise<void>((resolve) => {
        const handle = watch(
          keyPair,
          (keyPair) => {
            if (!keyPair) return

            handle.stop()
            resolve()
          },
          { immediate: true },
        )
      })
    }

    const array = new Uint8Array(await signDetached(payload, keyPair.value!))
    const signature = b64Encode(array, 0, array.length)
    const fingerprint = publicKeyID.value

    return `${SignatureVersionV1} ${identity.value} ${fingerprint} ${signature}`
  }

  onUnmounted(() => unregisterInterceptor.value?.())
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

function buildPayload(url: string, config: { headers?: Headers }) {
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
    headers,
    method: url.replace(/^\/api/, ''),
  }
}
