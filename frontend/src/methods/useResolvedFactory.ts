// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, type MaybeRefOrGetter, toValue } from 'vue'

import { useFeatures } from '@/methods/features'
import { useImageFactoryAuth } from '@/methods/useImageFactoryAuth'

// useResolvedFactory resolves a factory URL (as recorded on the selected Talos version) to the
// configured image factory that serves it. All assets of a given Talos version - image, PXE,
// installer, SBOM - must go to that factory and authenticate with its credentials, never mix in the
// primary factory by default.
export function useResolvedFactory(factoryURL: MaybeRefOrGetter<string | undefined>) {
  const { data: features } = useFeatures()
  const auth = useImageFactoryAuth(factoryURL)

  // The factories Omni currently has configured, primary first.
  const configuredFactories = computed(() => {
    const spec = features.value?.spec
    const list: { url: string; base: string; pxe?: string }[] = []

    if (spec?.image_factory_base_url) {
      list.push({
        url: spec.image_factory_base_url,
        base: spec.image_factory_base_url,
        pxe: spec.image_factory_pxe_base_url,
      })
    }

    if (spec?.secondary_image_factory_base_url) {
      list.push({
        url: spec.secondary_image_factory_base_url,
        base: spec.secondary_image_factory_base_url,
        pxe: spec.secondary_image_factory_pxe_base_url,
      })
    }

    return list
  })

  // The factory URL to resolve, falling back to the primary factory when the version does not record
  // one (older presets, or versions served before secondary factories existed).
  const resolvedURL = computed(
    () => toValue(factoryURL) || features.value?.spec.image_factory_base_url || '',
  )

  const resolvedFactory = computed(() =>
    configuredFactories.value.find((factory) => factory.url === resolvedURL.value),
  )

  // The version targets a factory URL that Omni no longer has configured.
  const orphaned = computed(
    () => features.value !== undefined && resolvedFactory.value === undefined,
  )

  const base = computed(
    () => resolvedFactory.value?.base ?? features.value?.spec.image_factory_base_url,
  )

  const pxe = computed(
    () => resolvedFactory.value?.pxe ?? features.value?.spec.image_factory_pxe_base_url,
  )

  // Credentials of the resolved factory, matched by its base URL. Both the image and the PXE links of
  // a factory authenticate with the same credentials, even though the PXE endpoint lives on a
  // different host.
  const credentials = computed(() => auth.value)

  return { base, pxe, credentials, orphaned }
}
