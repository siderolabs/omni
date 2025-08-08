// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import type { Ref } from 'vue'
import { shallowRef } from 'vue'

export type Notification = {
  props?: any
}

export const notification: Ref<Notification | null> = shallowRef(null)

export const showError = (title: string, body?: string, ref?: Ref<Notification | null>) => {
  console.error(`error occurred: title: ${title}, body: ${body}`)

  if (!ref) {
    ref = notification
  }

  ref.value = {
    props: {
      title: title,
      body: body,
      type: 'error',
      close: () => {
        ref.value = null
      },
    },
  }
}

export const showSuccess = (title: string, body?: string, ref?: Ref<Notification | null>) => {
  if (!ref) {
    ref = notification
  }

  ref.value = {
    props: {
      title: title,
      body: body,
      type: 'success',
      close: () => {
        ref.value = null
      },
    },
  }
}
