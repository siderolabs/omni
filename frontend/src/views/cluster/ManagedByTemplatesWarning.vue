<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <template v-if="clusterIsManagedByTemplates">
    <div v-if="warningStyle === 'alert'" class="pb-5">
      <t-alert type="warn" title="This cluster is managed using cluster templates.">
        It is recommended to manage it using its template and not through this UI.
      </t-alert>
    </div>
    <div v-else-if="warningStyle == 'popup'" class="text-xs pb-5">
      <p class="text-primary-P3 py-2">This cluster is managed using cluster templates.</p>
      <p class="text-primary-P3 py-2 font-bold">It is recommended to manage it using its template and not through this UI.</p>
    </div>
    <div v-else class="text-xs">
      <p class="text-primary-P3 py-2">Managed using cluster templates</p>
    </div>
  </template>
</template>

<script setup lang="ts">
import { useRoute } from "vue-router";
import { computed, ref, Ref } from "vue";
import { Resource } from "@/api/grpc";
import { ClusterSpec } from "@/api/omni/specs/omni.pb";
import Watch from "@/api/watch";
import { ClusterType, DefaultNamespace, ResourceManagedByClusterTemplates } from "@/api/resources";
import { Runtime } from "@/api/common/omni.pb";
import TAlert from "@/components/TAlert.vue";

export type WarningStyle = "alert" | "popup" | "short";

type Props = {
  cluster?: Resource<ClusterSpec>,
  warningStyle?: WarningStyle,
};

const props = withDefaults(defineProps<Props>(), {
  warningStyle: "alert",
});

const cluster: Ref<Resource<ClusterSpec> | undefined> = ref(props.cluster);

// If the cluster is not passed explicitly, watch it from the route, if cluster exists in the route params.
if (!props.cluster) {
  const route = useRoute();
  const clusterWatch = new Watch(cluster);

  clusterWatch.setup(computed(() => {
    if (!route.params.cluster) {
      return undefined;
    }

    return {
      resource: {
        type: ClusterType,
        namespace: DefaultNamespace,
        id: route.params.cluster as string,
      },
      runtime: Runtime.Omni,
    };
  }));
}

const clusterIsManagedByTemplates = computed(() => {
  return cluster.value?.metadata?.annotations?.[ResourceManagedByClusterTemplates] !== undefined;
});
</script>
