<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <t-actions-box style="height: 24px">
    <t-actions-box-item
      icon="log"
      @click="
        $router.push({
          name: 'MachineLogs',
          params: { machine: clusterMachineStatus.metadata.id! },
        })
      "
    >
      Logs
    </t-actions-box-item>
    <t-actions-box-item icon="copy" @click="copyMachineID">Copy Machine ID</t-actions-box-item>
    <t-actions-box-item icon="power" @click="shutdownNode" v-if="canRebootMachines"
      >Shutdown</t-actions-box-item
    >
    <t-actions-box-item icon="reboot" @click="rebootNode" v-if="canRebootMachines"
      >Reboot</t-actions-box-item
    >
    <t-actions-box-item
      v-if="
        clusterMachineStatus.spec.stage === ClusterMachineStatusSpecStage.BEFORE_DESTROY &&
        canAddClusterMachines
      "
      icon="rollback"
      @click="restoreNode"
    >
      Cancel Destroy
    </t-actions-box-item>
    <t-actions-box-item
      v-else-if="!deleteDisabled && canRemoveMachines"
      icon="delete"
      danger
      @click="deleteNode"
    >
      Destroy
    </t-actions-box-item>
  </t-actions-box>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import type { Resource } from '@/api/grpc'
import type { ClusterMachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { ClusterMachineStatusSpecStage } from '@/api/omni/specs/omni.pb'
import { copyText } from 'vue3-clipboard'
import { setupClusterPermissions } from '@/methods/auth'

import TActionsBox from '@/components/common/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/common/ActionsBox/TActionsBoxItem.vue'

const router = useRouter()
const props = defineProps<{
  clusterName: string
  deleteDisabled?: boolean
  clusterMachineStatus: Resource<ClusterMachineStatusSpec>
}>()

const { canRebootMachines, canAddClusterMachines, canRemoveMachines } = setupClusterPermissions({
  value: props.clusterName,
})

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
  copyText(props.clusterMachineStatus.metadata.id!, undefined, () => {})
}
</script>
