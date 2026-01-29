// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { onUnmounted, ref } from 'vue'

import { useFeatures } from '@/methods/features'

export function useDownloadImage() {
  const { data: features } = useFeatures()

  const abortController = ref<AbortController>()
  const isGenerating = ref(false)

  async function abort() {
    abortController.value?.abort()
  }

  async function download(imageUrl: string) {
    try {
      abort()

      abortController.value = new AbortController()
      isGenerating.value = true

      const headUrl = new URL(imageUrl, features.value?.spec.image_factory_base_url)

      const headResponse = await fetch(headUrl, {
        method: 'HEAD',
        signal: abortController.value.signal,
        headers: new Headers({
          'Cache-Control': 'no-store',
        }),
      })

      if (!headResponse.ok) {
        throw new Error(`request failed: ${headResponse.status} ${headResponse.statusText}`)
      }

      // URL format is hostname/image/:schematicID/:talosVersion/:image
      const [, , , talosVersion, imageName] = headUrl.pathname.split('/')

      if (!talosVersion || !imageName) {
        throw new Error(
          `invalid image path "${headUrl.pathname}". Got Talos version "${talosVersion}" and image name "${imageName}".`,
        )
      }

      const accountName = features.value?.spec.account?.name ?? 'default'

      const downloadUrl = new URL(headUrl)
      downloadUrl.searchParams.set('filename', `omni-${accountName}-${talosVersion}-${imageName}`)

      window.open(downloadUrl, '_self')
    } catch (e) {
      // Ignore abort errors
      if (abortController.value?.signal.aborted) return

      throw e
    } finally {
      isGenerating.value = false
    }
  }

  onUnmounted(() => {
    abort()
  })

  return {
    isGenerating,
    abort,
    download,
  }
}
