<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import type { Resource } from '@/api/grpc'
import type { ClusterMachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { ClusterMachineStatusSpecStage } from '@/api/omni/specs/omni.pb'
import TActionsBox from '@/components/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/ActionsBox/TActionsBoxItem.vue'
import NodeRebootModal from '@/components/Modals/NodeRebootModal.vue'
import { useClusterPermissions } from '@/methods/auth'

const router = useRouter()
const { copy } = useClipboard()

const { clusterName, clusterMachineStatus } = defineProps<{
  clusterName: string
  deleteDisabled?: boolean
  clusterMachineStatus: Resource<ClusterMachineStatusSpec>
}>()

const nodeRebootModalOpen = ref(false)

const { canRebootMachines, canAddClusterMachines, canRemoveMachines } = useClusterPermissions(
  () => clusterName,
)

const deleteNode = () => {
  router.push({
    query: {
      modal: 'nodeDestroy',
      cluster: clusterName,
      machine: clusterMachineStatus.metadata.id,
      goback: 'true',
    },
  })
}

const shutdownNode = () => {
  router.push({
    query: {
      modal: 'shutdown',
      cluster: clusterName,
      machine: clusterMachineStatus.metadata.id,
      goback: 'true',
    },
  })
}

const restoreNode = () => {
  router.push({
    query: {
      modal: 'nodeDestroyCancel',
      cluster: clusterName,
      machine: clusterMachineStatus.metadata.id,
      goback: 'true',
    },
  })
}

const copyMachineID = () => {
  copy(clusterMachineStatus.metadata.id!)
}
</script>

<template>
  <TActionsBox>
    <TActionsBoxItem
      icon="log"
      @select="
        $router.push({
          name: 'MachineLogs',
          params: { machine: clusterMachineStatus.metadata.id! },
        })
      "
    >
      Logs
    </TActionsBoxItem>
    <TActionsBoxItem icon="copy" @select="copyMachineID">Copy Machine ID</TActionsBoxItem>
    <TActionsBoxItem v-if="canRebootMachines" icon="power" @select="shutdownNode">
      Shutdown
    </TActionsBoxItem>
    <TActionsBoxItem v-if="canRebootMachines" icon="reboot" @select="nodeRebootModalOpen = true">
      Reboot
    </TActionsBoxItem>
    <TActionsBoxItem
      v-if="
        clusterMachineStatus.spec.stage === ClusterMachineStatusSpecStage.BEFORE_DESTROY &&
        canAddClusterMachines
      "
      icon="rollback"
      @select="restoreNode"
    >
      Cancel Destroy
    </TActionsBoxItem>
    <TActionsBoxItem
      v-else-if="!deleteDisabled && canRemoveMachines"
      icon="delete"
      danger
      @select="deleteNode"
    >
      Destroy
    </TActionsBoxItem>
  </TActionsBox>

  <!-- v-if on modals as there may be many menus mounting many modals -->
  <NodeRebootModal
    v-if="nodeRebootModalOpen"
    v-model:open="nodeRebootModalOpen"
    :cluster-id="clusterName"
    :machine-id="clusterMachineStatus.metadata.id!"
  />
</template>
