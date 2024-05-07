// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { Ref, Component, shallowRef } from 'vue';

export type Modal = {
  component: Component,
  props?: Object,
};

export const modal: Ref<{component: Component, props: any} | null> = shallowRef(null);

export const showModal = (component: Component, props: any) => {
  modal.value = {
    component: component,
    props: props,
  }
};

export const closeModal = () => {
  modal.value = null;
};
