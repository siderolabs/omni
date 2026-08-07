<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import { ref } from 'vue'

import type { Resource } from '@/api/grpc'
import type { ClusterMachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { ClusterMachineStatusSpecStage } from '@/api/omni/specs/omni.pb'
import TActionsBox from '@/components/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/ActionsBox/TActionsBoxItem.vue'
import NodeRebootModal from '@/components/Modals/NodeRebootModal.vue'
import NodeRemoveCancelModal from '@/components/Modals/NodeRemoveCancelModal.vue'
import NodeRemoveModal from '@/components/Modals/NodeRemoveModal.vue'
import NodeShutdownModal from '@/components/Modals/NodeShutdownModal.vue'
import { useClusterPermissions } from '@/methods/auth'

const { copy } = useClipboard()

const { clusterName, clusterMachineStatus } = defineProps<{
  clusterName: string
  removeDisabled?: boolean
  clusterMachineStatus: Resource<ClusterMachineStatusSpec>
}>()

const nodeShutdownModalOpen = ref(false)
const nodeRebootModalOpen = ref(false)
const nodeRemoveModalOpen = ref(false)
const nodeRemoveCancelModalOpen = ref(false)

const { canRebootMachines, canAddClusterMachines, canRemoveMachines } = useClusterPermissions(
  () => clusterName,
)

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
    <TActionsBoxItem v-if="canRebootMachines" icon="power" @select="nodeShutdownModalOpen = true">
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
      @select="nodeRemoveCancelModalOpen = true"
    >
      Cancel Remove
    </TActionsBoxItem>
    <TActionsBoxItem
      v-else-if="!removeDisabled && canRemoveMachines"
      icon="delete"
      danger
      @select="nodeRemoveModalOpen = true"
    >
      Remove
    </TActionsBoxItem>
  </TActionsBox>

  <!-- v-if on modals as there may be many menus mounting many modals -->
  <NodeShutdownModal
    v-if="nodeShutdownModalOpen"
    v-model:open="nodeShutdownModalOpen"
    :cluster-id="clusterName"
    :machine-id="clusterMachineStatus.metadata.id!"
  />

  <NodeRebootModal
    v-if="nodeRebootModalOpen"
    v-model:open="nodeRebootModalOpen"
    :cluster-id="clusterName"
    :machine-id="clusterMachineStatus.metadata.id!"
  />

  <NodeRemoveModal
    v-if="nodeRemoveModalOpen"
    v-model:open="nodeRemoveModalOpen"
    :cluster-id="clusterName"
    :machine-id="clusterMachineStatus.metadata.id!"
  />

  <NodeRemoveCancelModal
    v-if="nodeRemoveCancelModalOpen"
    v-model:open="nodeRemoveCancelModalOpen"
    :machine-id="clusterMachineStatus.metadata.id!"
  />
</template>
