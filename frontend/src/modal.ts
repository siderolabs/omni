// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Component } from 'vue'
import { shallowRef } from 'vue'
import type { ComponentProps } from 'vue-component-type-helpers'

export const modal = shallowRef<{ component: Component; props: unknown } | null>(null)

export const showModal = <C extends Component>(component: C, props: ComponentProps<C>) => {
  modal.value = {
    component: component,
    props: props,
  }
}

export const closeModal = () => {
  modal.value = null
}
