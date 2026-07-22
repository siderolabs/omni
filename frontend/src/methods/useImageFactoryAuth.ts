// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, type MaybeRefOrGetter, toValue } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { ImageFactoryAuthSpec } from '@/api/omni/specs/virtual.pb'
import { ImageFactoryAuthID, ImageFactoryAuthType, VirtualNamespace } from '@/api/resources'
import { useResourceGet } from '@/methods/useResourceGet'

export function useImageFactoryAuth(factoryURL?: MaybeRefOrGetter<string | undefined>) {
  const { data } = useResourceGet<ImageFactoryAuthSpec>(() => ({
    runtime: Runtime.Omni,
    resource: {
      namespace: VirtualNamespace,
      type: ImageFactoryAuthType,
      id: toValue(factoryURL) ?? ImageFactoryAuthID,
    },
  }))

  return computed(() => data.value?.spec)
}

export interface FactoryCredentials {
  username?: string
  password?: string
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
