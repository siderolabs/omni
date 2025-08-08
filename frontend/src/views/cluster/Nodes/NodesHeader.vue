<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex justify-between items-start flex-wrap mb-7">
    <t-breadcrumbs :nodeName="nodeName" />
    <div class="flex">
      <t-button
        class="header-button"
        icon="power"
        iconPosition="left"
        type="secondary"
        @click="shutdownNode"
        :disabled="!canRebootMachines"
        >Shutdown</t-button
      >
      <t-button
        class="header-button"
        icon="reboot"
        iconPosition="left"
        type="secondary"
        @click="rebootNode"
        :disabled="!canRebootMachines"
        >Reboot</t-button
      >
      <t-button
        v-if="machineSetNode"
        class="header-button delete-button"
        icon="delete"
        iconPosition="left"
        type="secondary"
        @click="destroyNode"
        :disabled="!canRemoveMachines"
        >Destroy</t-button
      >
      <t-button
        v-else-if="!loading"
        class="header-button"
        icon="rollback"
        iconPosition="left"
        type="secondary"
        @click="restoreNode"
        :disabled="!canAddClusterMachines"
        >Cancel Destroy</t-button
      >
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import type { Ref } from 'vue'
import { computed, onMounted, ref } from 'vue'
import { setupClusterPermissions } from '@/methods/auth'

import TBreadcrumbs from '@/components/TBreadcrumbs.vue'
import TButton from '@/components/common/Button/TButton.vue'
import Watch from '@/api/watch'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import { DefaultNamespace, MachineSetNodeType, MachineStatusType } from '@/api/resources'
import { Runtime } from '@/api/common/omni.pb'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'

const route = useRoute()
const router = useRouter()

const nodeName = ref(route.params.machine as string)

onMounted(async () => {
  const res: Resource<MachineStatusSpec> = await ResourceService.Get(
    {
      namespace: DefaultNamespace,
      type: MachineStatusType,
      id: route.params.machine! as string,
    },
    withRuntime(Runtime.Omni),
  )

  nodeName.value = res.spec.network?.hostname || res.metadata.id!
})

const shutdownNode = () => {
  router.push({
    query: {
      modal: 'shutdown',
      machine: route.params.machine,
      ...route.query,
    },
  })
}

const rebootNode = () => {
  router.push({
    query: {
      modal: 'reboot',
      machine: route.params.machine,
      ...route.query,
    },
  })
}

const machineSetNode: Ref<Resource | undefined> = ref()
const watch = new Watch(machineSetNode)
watch.setup({
  resource: {
    type: MachineSetNodeType,
    id: route.params.machine as string,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni,
})

const loading = watch.loading

const destroyNode = async () => {
  router.push({
    query: {
      modal: 'nodeDestroy',
      machine: route.params.machine,
      cluster: route.params.cluster,
    },
  })
}

const restoreNode = async () => {
  router.push({
    query: {
      modal: 'nodeDestroyCancel',
      machine: route.params.machine,
      cluster: route.params.cluster,
    },
  })
}

const { canRebootMachines, canRemoveMachines, canAddClusterMachines } = setupClusterPermissions(
  computed(() => route.params.cluster as string),
)
</script>

<style scoped>
.header-button:first-child {
  border-radius: 4px 0 0 4px;
}

.header-button:not(:last-child) {
  border-right: none;
}

.header-button:last-child {
  border-radius: 0 4px 4px 0;
}

.header-button:not(:first-child):not(:last-child) {
  border-radius: 0;
}

.delete-button {
  @apply text-red-R1 hover:text-red-R1 active:text-red-R1;
}
</style>
