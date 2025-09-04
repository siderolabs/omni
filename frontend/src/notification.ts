// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { toast } from 'vue-sonner'

export function showError(title: string, description?: string) {
  console.error(`error occurred: title: ${title}, body: ${description}`)

  toast.error(title, { description, duration: Infinity })
}

export function showSuccess(title: string, description?: string) {
  toast.success(title, { description, duration: 5000 })
}
