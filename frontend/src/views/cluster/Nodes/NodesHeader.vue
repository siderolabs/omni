<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import { DefaultNamespace, MachineSetNodeType, MachineStatusType } from '@/api/resources'
import Watch from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import TBreadcrumbs from '@/components/TBreadcrumbs.vue'
import { setupClusterPermissions } from '@/methods/auth'

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

<template>
  <div class="mb-7 flex flex-wrap items-start justify-between">
    <TBreadcrumbs :node-name="nodeName" />
    <div class="flex">
      <TButton
        class="header-button"
        icon="power"
        icon-position="left"
        type="secondary"
        :disabled="!canRebootMachines"
        @click="shutdownNode"
      >
        Shutdown
      </TButton>
      <TButton
        class="header-button"
        icon="reboot"
        icon-position="left"
        type="secondary"
        :disabled="!canRebootMachines"
        @click="rebootNode"
      >
        Reboot
      </TButton>
      <TButton
        v-if="machineSetNode"
        class="header-button delete-button"
        icon="delete"
        icon-position="left"
        type="secondary"
        :disabled="!canRemoveMachines"
        @click="destroyNode"
      >
        Destroy
      </TButton>
      <TButton
        v-else-if="!loading"
        class="header-button"
        icon="rollback"
        icon-position="left"
        type="secondary"
        :disabled="!canAddClusterMachines"
        @click="restoreNode"
      >
        Cancel Destroy
      </TButton>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

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
  @apply text-red-r1 hover:text-red-r1 active:text-red-r1;
}
</style>
