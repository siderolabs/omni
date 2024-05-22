<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <patches :machine="machine" v-if="machine"/>
</template>

<script setup lang="ts">
import { Runtime } from '@/api/common/omni.pb';
import { Resource } from '@/api/grpc';
import { DefaultNamespace, MachineStatusType } from '@/api/resources';
import { computed, ref } from 'vue';
import { useRoute } from 'vue-router';

import Watch from '@/api/watch';
import Patches from '@/views/cluster/Config/Patches.vue';

const machine = ref<Resource>();
const route = useRoute();

const machineWatch = new Watch(machine);

machineWatch.setup(computed(() => {
  return {
    resource: {
      id: route.params.machine as string,
      type: MachineStatusType,
      namespace: DefaultNamespace,
    },
    runtime: Runtime.Omni
  }
}));
</script>
