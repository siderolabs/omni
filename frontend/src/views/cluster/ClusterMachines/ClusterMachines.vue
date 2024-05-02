<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div>
    <watch v-for="id in watches" :opts="{
      resource: { namespace: DefaultNamespace, type: MachineSetStatusType, id: id },
      runtime: Runtime.Omni,
    }" spinner :key="id">
      <template #default="{ items }">
        <machine-set v-for="item in items" :key="itemID(item)" :machineSet="item" :id="item.metadata.id"/>
      </template>
    </watch>
  </div>
</template>

<script setup lang="ts">
import { toRefs, ref, computed, Ref } from "vue";
import {
  DefaultNamespace,
  MachineSetStatusType,
  LabelCluster,
  LabelControlPlaneRole,
  MachineSetNodeType, MachineSetType,
} from "@/api/resources";
import { Runtime } from "@/api/common/omni.pb";
import WatchResource, { itemID } from "@/api/watch";
import { Resource } from "@/api/grpc";
import { MachineSetSpec } from "@/api/omni/specs/omni.pb";
import { sortMachineSetIds } from "@/methods/machineset";

import Watch from "@/components/common/Watch/Watch.vue";
import MachineSet from "./MachineSet.vue";

const props = defineProps<{
  clusterID: string,
}>();

const { clusterID } = toRefs(props);

const machineSets: Ref<Resource<MachineSetSpec>[]> = ref([]);
const machineSetsWatch = new WatchResource(machineSets);
machineSetsWatch.setup(computed(() => {
  return {
    resource: {
      type: MachineSetType,
      namespace: DefaultNamespace,
    },
    runtime: Runtime.Omni,
    selectors: [`${LabelCluster}=${clusterID.value}`]
  };
}))

const watches = computed(() => sortMachineSetIds(clusterID.value, machineSets.value.map(machineSet => machineSet?.metadata?.id ?? "")));

const controlPlaneNodes = ref<Resource[]>([]);

const machineNodesWatch = new WatchResource(controlPlaneNodes);
machineNodesWatch.setup(computed(() => {
  return {
    resource: {
      type: MachineSetNodeType,
      namespace: DefaultNamespace,
    },
    runtime: Runtime.Omni,
    selectors: [
      `${LabelControlPlaneRole}`
    ]
  };
}));
</script>
