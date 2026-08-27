// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, type MaybeRefOrGetter, toValue } from 'vue'

import { useFeatures } from '@/methods/features'

// useResolvedFactory resolves a factory URL (as recorded on the selected Talos version) to the
// configured image factory that serves it. All assets of a given Talos version - image, PXE,
// installer, SBOM - must go to that factory and authenticate with its credentials, never mix in the
// primary factory by default.
export function useResolvedFactory(factoryURL?: MaybeRefOrGetter<string | undefined>) {
  const { data: features } = useFeatures()

  // The factories Omni currently has configured, primary first.
  const configuredFactories = computed(() => {
    const spec = features.value?.spec
    const list: {
      base: string
      pxe?: string
      requiresAuth?: boolean
    }[] = []

    if (spec?.image_factory_base_url) {
      list.push({
        base: spec.image_factory_base_url,
        pxe: spec.image_factory_pxe_base_url,
        requiresAuth: spec.image_factory_requires_auth,
      })
    }

    if (spec?.secondary_image_factory_base_url) {
      list.push({
        base: spec.secondary_image_factory_base_url,
        pxe: spec.secondary_image_factory_pxe_base_url,
        requiresAuth: spec.secondary_image_factory_requires_auth,
      })
    }

    return list
  })

  // The factory URL to resolve, falling back to the primary factory when the version does not record
  // one (older presets, or versions served before secondary factories existed).
  const resolvedFactory = computed(() => {
    const url = toValue(factoryURL)
    if (!url) return configuredFactories.value.at(0)

    return configuredFactories.value.find((factory) => factory.base === url)
  })

  const url = computed(() => resolvedFactory.value?.base)
  const pxeUrl = computed(() => resolvedFactory.value?.pxe)
  const requiresAuth = computed(() => resolvedFactory.value?.requiresAuth)

  return { url, pxeUrl, requiresAuth }
}
