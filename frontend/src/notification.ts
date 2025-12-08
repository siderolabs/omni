// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { milliseconds } from 'date-fns'
import type { Component } from 'vue'
import { type ExternalToast, toast } from 'vue-sonner'

type TitleOrComponent = (() => string | Component) | string | Component

export function showError(title: TitleOrComponent, descOrOpts?: string | ExternalToast) {
  console.error(`error occurred: title: ${title}, body: ${descOrOpts}`)

  showToast('error', title, milliseconds({ minutes: 1 }), descOrOpts)
}

export function showSuccess(title: TitleOrComponent, descOrOpts?: string | ExternalToast) {
  showToast('success', title, milliseconds({ seconds: 5 }), descOrOpts)
}

export function showWarning(title: TitleOrComponent, descOrOpts?: string | ExternalToast) {
  showToast('warning', title, milliseconds({ seconds: 5 }), descOrOpts)
}

function showToast(
  type: 'error' | 'success' | 'warning',
  title: TitleOrComponent,
  duration: number,
  descOrOpts?: string | ExternalToast,
) {
  const description = typeof descOrOpts === 'string' ? descOrOpts : undefined
  const options = typeof descOrOpts !== 'string' ? descOrOpts : undefined

  toast[type](title, { description, duration, ...options })
}
