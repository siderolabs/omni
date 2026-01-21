// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useLocalStorage } from '@vueuse/core'

export function useIdentity() {
  const identityRef = useLocalStorage<string>('identity', null)
  const fullnameRef = useLocalStorage<string>('fullname', null)
  const avatarRef = useLocalStorage<string>('avatar', null)

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
