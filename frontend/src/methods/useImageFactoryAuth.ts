// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, effectScope } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { ImageFactoryAuthSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, ImageFactoryAuthType } from '@/api/resources'

import { useResourceList } from './useResourceList'

let auth: ReturnType<typeof initImageFactoryAuth> | undefined

export function useImageFactoryAuth() {
  auth ||= initImageFactoryAuth()

  return auth
}

function initImageFactoryAuth() {
  return effectScope(true).run(() => {
    const { data } = useResourceList<ImageFactoryAuthSpec>({
      runtime: Runtime.Omni,
      resource: {
        namespace: DefaultNamespace,
        type: ImageFactoryAuthType,
      },
    })

    return computed(() => data.value[0]?.spec)
  })!
}

// withImageFactoryAuth embeds basic auth credentials into a URL's userinfo so
// that top-level browser navigation (anchor clicks) sends them. Useful only for
// links the browser navigates to directly.
export function withImageFactoryAuth(url: string, spec?: ImageFactoryAuthSpec): string {
  if (!spec?.username || !spec?.password) {
    return url
  }

  const u = new URL(url)
  u.username = spec.username
  u.password = spec.password

  return u.toString()
}
