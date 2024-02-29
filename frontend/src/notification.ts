// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { Ref, shallowRef } from 'vue';

export type Notification = {
  props?: any,
};

export const notification: Ref<Notification | null> = shallowRef(null);

export const showError = (title: string, body?: string) => {
  console.error(`error occurred: title: ${title}, body: ${body}`)

  notification.value = {
    props: {
      title: title,
      body: body,
      type: "error",
      close: () => {
        notification.value = null;
      }
    }
  }
};

export const showSuccess = (title: string, body?: string) => {
  notification.value = {
    props: {
      title: title,
      body: body,
      type: "success",
      close: () => {
        notification.value = null;
      }
    }
  }
};
