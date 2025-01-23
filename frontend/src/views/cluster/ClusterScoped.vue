<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <component v-if="cluster" :is="inner" :currentCluster="cluster"/>
  <div v-else-if="bootstrapped" class="flex-1 font-sm">
    <t-alert title="Cluster Not Found" type="error">
      Cluster {{ route.params.cluster as string }} does not exist.
    </t-alert>
  </div>
</template>

<script setup lang="ts">
import { type Component, computed, ref, type Ref } from "vue";
import { Resource } from "@/api/grpc";
import { ClusterSpec } from "@/api/omni/specs/omni.pb";
import Watch from "@/api/watch";
import { getContext } from "@/context";
import { Runtime } from "@/api/common/omni.pb";
import { ClusterType, DefaultNamespace } from "@/api/resources";
import { useRoute } from "vue-router";
import TAlert from "@/components/TAlert.vue";
import { EventType } from "@/api/omni/resources/resources.pb";

const bootstrapped = ref(false);
const cluster: Ref<Resource<ClusterSpec> | undefined> = ref();

const watch = new Watch(cluster, message => {
  if (message.event?.event_type === EventType.BOOTSTRAPPED) {
    bootstrapped.value = true;
  }
});

const route = useRoute();
const context = getContext(route);

watch.setup(computed(() => {
  return {
    runtime: Runtime.Omni,
    resource: {
      type: ClusterType,
      namespace: DefaultNamespace,
      id: route.params.cluster as string,
    },
    context
  };
}));

type Props = {
  inner: Component,
};

defineProps<Props>();
</script>
