<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { DefaultNamespace, MachineStatusType } from '@/api/resources'
import Watch from '@/components/common/Watch/Watch.vue'

import Patches from '../Config/Patches.vue'

const route = useRoute()

const opts = computed(() => {
  return {
    resource: {
      id: route.params.machine as string,
      type: MachineStatusType,
      namespace: DefaultNamespace,
    },
    runtime: Runtime.Omni,
  }
})
</script>

<template>
  <Watch spinner :opts="opts">
    <template #default="{ data }">
      <Patches :machine="data" />
    </template>
  </Watch>
</template>
