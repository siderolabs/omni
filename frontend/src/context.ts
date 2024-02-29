// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { ref } from 'vue';
import { useRoute } from 'vue-router';
import { WatchContext } from '@/api/watch';

export namespace context {
  export const current:any = ref(localStorage.context ? JSON.parse(localStorage.context) : null);

  export const capabilities = {
    capi: ref(false),
    sidero: ref(false),
    packet: ref(false),
  };
}

export function changeContext(c: Object) {
  localStorage.context = JSON.stringify(c);

  context.current.value = c;
}

export function getContext(route: any = null): WatchContext {
  route = route || useRoute();

  const cluster = clusterName();

  const res: WatchContext = {
    cluster: cluster || "",
  };

  const machine = route.params.machine ?? route.query.machine;
  if (machine) {
    res.nodes = [machine];
  }

  return res;
}

export function clusterName(): string | null {
  const route = useRoute();

  if (route && route.params.cluster) {
    return route.params.cluster as string;
  }

  if (route && route.query.cluster) {
    return route.query.cluster as string;
  }

  return context.current.value ? context.current.value.name : null;
}