// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { milliseconds } from 'date-fns'
import { toast } from 'vue-sonner'

export function showError(title: string, description?: string) {
  console.error(`error occurred: title: ${title}, body: ${description}`)

  toast.error(title, { description, duration: milliseconds({ minutes: 1 }) })
}

export function showSuccess(title: string, description?: string) {
  toast.success(title, { description, duration: milliseconds({ seconds: 5 }) })
}
