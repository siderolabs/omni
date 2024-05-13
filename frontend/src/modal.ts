// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { Ref, Component, shallowRef, AllowedComponentProps, VNodeProps } from 'vue';

type ComponentProps<C extends Component> = C extends new (...args: any) => any
  ? Omit<InstanceType<C>['$props'], keyof VNodeProps | keyof AllowedComponentProps>
  : never;

export type Modal = {
  component: Component,
  props?: any,
};

export const modal: Ref<{component: Component, props: any} | null> = shallowRef(null);

export const showModal = <C extends Component>(component: C, props: ComponentProps<C>) => {
  modal.value = {
    component: component,
    props: props,
  }
};

export const closeModal = () => {
  modal.value = null;
};
