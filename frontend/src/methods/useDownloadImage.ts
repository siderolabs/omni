// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, onUnmounted, ref } from 'vue'

import { useFeatures } from '@/methods/features'

import { formatBytes } from '.'

export enum Phase {
  Idle = 0,
  Generating = 1,
  Loading = 2,
}

export function useDownloadImage() {
  const { data: features } = useFeatures()

  const abortController = ref<AbortController>()
  const phase = ref(Phase.Idle)
  const error = ref<Error>()
  const bytesDownloaded = ref(0)
  const bytesDownloadedFormatted = computed(() => formatBytes(bytesDownloaded.value))

  async function abort() {
    abortController.value?.abort()
  }

  async function download(imageUrl: string) {
    try {
      abort()

      abortController.value = new AbortController()
      bytesDownloaded.value = 0
      error.value = undefined
      phase.value = Phase.Generating

      const factoryUrl = new URL(imageUrl, features.value?.spec.image_factory_base_url)

      await doRequest(factoryUrl, {
        signal: abortController.value.signal,
        method: 'HEAD',
        headers: new Headers({ 'Cache-Control': 'no-store' }),
      })

      phase.value = Phase.Loading

      const resp = await doRequest(factoryUrl, { signal: abortController.value.signal })

      const res = new Response(
        new ReadableStream({
          async start(controller) {
            const reader = resp.body!.getReader()

            while (true) {
              const { done, value } = await reader.read()
              if (done) break

              bytesDownloaded.value += value.byteLength
              controller.enqueue(value)
            }

            controller.close()
          },
        }),
      )

      const objectURL = window.URL.createObjectURL(await res.blob())

      const [, , , talosVersion, imageName] = factoryUrl.pathname.split('/')

      const a = document.createElement('a')
      a.style.display = 'none'
      a.href = objectURL
      a.download = `omni-${features.value?.spec.account_name}-${talosVersion}-${imageName}`
      document.body.appendChild(a)
      a.click()

      window.URL.revokeObjectURL(objectURL)
      a.remove()
    } catch (e) {
      // Ignore abort errors
      if (abortController.value?.signal.aborted) return

      error.value = e instanceof Error ? e : new Error(String(e))
    } finally {
      phase.value = Phase.Idle
    }
  }

  onUnmounted(() => {
    abort()
  })

  return {
    phase,
    bytesDownloaded,
    bytesDownloadedFormatted,
    error,
    abort,
    download,
  }
}

async function doRequest(...[url, init]: Parameters<typeof fetch>) {
  const resp = await fetch(url, init)

  if (!resp.ok) {
    throw new Error(`request failed: ${resp.status} ${await resp.text()}`)
  }

  return resp
}
