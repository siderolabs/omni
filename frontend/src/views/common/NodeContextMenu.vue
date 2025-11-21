<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import { computed } from 'vue'
import { useRouter } from 'vue-router'

import type { Resource } from '@/api/grpc'
import type { ClusterMachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { ClusterMachineStatusSpecStage } from '@/api/omni/specs/omni.pb'
import TActionsBox from '@/components/common/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/common/ActionsBox/TActionsBoxItem.vue'
import { setupClusterPermissions } from '@/methods/auth'

const router = useRouter()
const { copy } = useClipboard()

const props = defineProps<{
  clusterName: string
  deleteDisabled?: boolean
  clusterMachineStatus: Resource<ClusterMachineStatusSpec>
}>()

const { canRebootMachines, canAddClusterMachines, canRemoveMachines } = setupClusterPermissions(
  computed(() => props.clusterName),
)

const deleteNode = () => {
  router.push({
    query: {
      modal: 'nodeDestroy',
      cluster: props.clusterName,
      machine: props.clusterMachineStatus.metadata.id,
      goback: 'true',
    },
  })
}

const shutdownNode = () => {
  router.push({
    query: {
      modal: 'shutdown',
      cluster: props.clusterName,
      machine: props.clusterMachineStatus.metadata.id,
      goback: 'true',
    },
  })
}

const rebootNode = () => {
  router.push({
    query: {
      modal: 'reboot',
      cluster: props.clusterName,
      machine: props.clusterMachineStatus.metadata.id,
      goback: 'true',
    },
  })
}

const restoreNode = () => {
  router.push({
    query: {
      modal: 'nodeDestroyCancel',
      cluster: props.clusterName,
      machine: props.clusterMachineStatus.metadata.id,
      goback: 'true',
    },
  })
}

const copyMachineID = () => {
  copy(props.clusterMachineStatus.metadata.id!)
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
    <TActionsBoxItem v-if="canRebootMachines" icon="reboot" @select="rebootNode">
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
</template>
