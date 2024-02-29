<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col gap-2">
    <t-breadcrumbs :last="`${machine} Logs`"/>
    <machine-logs-container class="flex-1"/>
  </div>
</template>

<script setup lang="ts">
import { Runtime } from "@/api/common/omni.pb";
import { Resource, ResourceService } from "@/api/grpc";
import { MachineStatusSpec } from "@/api/omni/specs/omni.pb";
import { withRuntime } from "@/api/options";
import { DefaultNamespace, MachineStatusType } from "@/api/resources";
import TBreadcrumbs from "@/components/TBreadcrumbs.vue";
import MachineLogsContainer from "@/views/omni/Machines/MachineLogsContainer.vue";
import { onBeforeMount, ref, watch } from "vue";
import { useRoute } from "vue-router";

const route = useRoute();

const getMachineName = async () => {
  const res: Resource<MachineStatusSpec> = await ResourceService.Get({
    namespace: DefaultNamespace,
    type: MachineStatusType,
    id: route.params.machine! as string,
  }, withRuntime(Runtime.Omni));

  machine.value = res.spec.network?.hostname || res.metadata.id!;
};

const machine = ref(route.params.machine);

onBeforeMount(getMachineName);
watch(() => route.params, getMachineName)
</script>
