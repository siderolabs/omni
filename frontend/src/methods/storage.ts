// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { customRef } from 'vue'

/**
 * @deprecated use useSessionStorage from @vueuse
 */
export default <T>(storage: Storage, key: string, defaultValue: T) => {
  return customRef<T>((track, trigger) => ({
    get: () => {
      track()

      const value = storage.getItem(key)

      return value ? JSON.parse(value) : defaultValue
    },
    set: (value) => {
      if (value === null) {
        storage.removeItem(key)
      } else {
        storage.setItem(key, JSON.stringify(value))
      }

      trigger()
    },
  }))
}
