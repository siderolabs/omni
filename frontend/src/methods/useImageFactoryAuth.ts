// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, type MaybeRefOrGetter, toValue } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { ImageFactoryAuthSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, ImageFactoryAuthType } from '@/api/resources'
import { useFeatures } from '@/methods/features'
import { useResourceGet } from '@/methods/useResourceGet'

export interface FactoryCredentials {
  username?: string
  password?: string
}

// ImageFactoryAuth resources are keyed by the factory URL with any trailing slash stripped.
function trimTrailingSlash(url: string): string {
  return url.replace(/\/+$/, '')
}

// useImageFactoryAuth reads the credentials of a specific image factory, identified by its URL.
// It falls back to the primary factory when no URL is given, and resolves to undefined for a
// factory that has no credentials configured (i.e. a public one).
export function useImageFactoryAuth(factoryURL?: MaybeRefOrGetter<string | undefined>) {
  const { data: features } = useFeatures()

  const id = computed(() => {
    const url = toValue(factoryURL) || features.value?.spec.image_factory_base_url

    return url ? trimTrailingSlash(url) : undefined
  })

  const { data } = useResourceGet<ImageFactoryAuthSpec>(() => ({
    runtime: Runtime.Omni,
    resource: {
      namespace: DefaultNamespace,
      type: ImageFactoryAuthType,
      id: id.value ?? '',
    },
    skip: id.value === undefined,
  }))

  return computed(() => data.value?.spec)
}

// withImageFactoryAuth embeds basic auth credentials into a URL's userinfo so
// that top-level browser navigation (anchor clicks) sends them. Useful only for
// links the browser navigates to directly.
export function withImageFactoryAuth(url: string, credentials?: FactoryCredentials): string {
  if (!credentials?.username || !credentials?.password) {
    return url
  }

  const u = new URL(url)
  u.username = credentials.username
  u.password = credentials.password

  return u.toString()
}
