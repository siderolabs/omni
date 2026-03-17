// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useLocalStorage } from '@vueuse/core'

const identityRef = useLocalStorage<string>('identity', null, { flush: 'sync' })
const fullnameRef = useLocalStorage<string>('fullname', null, { flush: 'sync' })
const avatarRef = useLocalStorage<string>('avatar', null, { flush: 'sync' })

export function useIdentity() {
  return {
    identity: identityRef,
    fullname: fullnameRef,
    avatar: avatarRef,
    clear() {
      identityRef.value = null
      fullnameRef.value = null
      avatarRef.value = null
    },
  }
}
