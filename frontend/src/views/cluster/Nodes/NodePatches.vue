<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, MachineStatusType } from '@/api/resources'
import { useResourceWatch } from '@/methods/useResourceWatch'
import Patches from '@/views/cluster/Config/Patches.vue'

const route = useRoute()

const { data: machine } = useResourceWatch<MachineStatusSpec>(() => ({
  resource: {
    id: route.params.machine.toString(),
    type: MachineStatusType,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni,
}))
</script>

<template>
  <Patches v-if="machine" :machine class="py-4" />
</template>
