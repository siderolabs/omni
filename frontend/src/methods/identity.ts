// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { StorageSerializers, useLocalStorage, type UseStorageOptions } from '@vueuse/core'

const storageOptions: UseStorageOptions<string> = {
  serializer: StorageSerializers.string,
  writeDefaults: false,
}

const identityRef = useLocalStorage('identity', '', storageOptions)
const fullnameRef = useLocalStorage('fullname', '', storageOptions)
const avatarRef = useLocalStorage('avatar', '', storageOptions)

export function useIdentity() {
  return {
    identity: identityRef,
    fullname: fullnameRef,
    avatar: avatarRef,
    clear() {
      identityRef.value = ''
      fullnameRef.value = ''
      avatarRef.value = ''
    },
  }
}
