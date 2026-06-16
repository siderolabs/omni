<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import { computed, ref, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { RequestError } from '@/api/fetch.pb'
import { Code } from '@/api/google/rpc/code.pb'
import { type Resource, ResourceService } from '@/api/grpc'
import type { DeleteRequest } from '@/api/omni/resources/resources.pb'
import type {
  ClusterMachineStatusSpec,
  MachineSetNodeSpec,
  NodeForceDestroyRequestSpec,
} from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import {
  ClusterMachineStatusLabelNodeName,
  ClusterMachineStatusType,
  DefaultNamespace,
  LabelCluster,
  LabelControlPlaneRole,
  LabelMachineSet,
  MachineSetNodeType,
  NodeForceDestroyRequestType,
  SiderolinkResourceType,
} from '@/api/resources'
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import ManagedByTemplatesWarning from '@/components/ManagedByTemplatesWarning.vue'
import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { ClusterCommandError, destroyResources, teardownResources } from '@/methods/cluster'
import { controlPlaneMachineSetId } from '@/methods/machineset'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError, showSuccess } from '@/notification'

const { clusterId, machineId } = defineProps<{
  clusterId: string
  machineId: string
}>()

const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{
  onDestroy: []
}>()

const isDestroying = ref(false)
const forceEtcdLeave = ref(false)
const forceDestroy = ref(false)

watchEffect(() => {
  if (open.value) return

  isDestroying.value = false
  forceEtcdLeave.value = false
  forceDestroy.value = false
})

const {
  data: clusterMachineStatus,
  loading: clusterMachineStatusLoading,
  err: clusterMachineStatusErr,
} = useResourceWatch<ClusterMachineStatusSpec>(() => ({
  skip: !open.value,
  runtime: Runtime.Omni,
  resource: {
    type: ClusterMachineStatusType,
    namespace: DefaultNamespace,
    id: machineId,
  },
}))

const {
  data: machineSetNodes,
  loading: machineSetNodesLoading,
  err: machineSetNodesErr,
} = useResourceWatch<MachineSetNodeSpec>(() => ({
  skip: !open.value,
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: MachineSetNodeType,
  },
  selectors: [`${LabelCluster}=${clusterId}`],
}))

const currentMachineSetNode = computed(() =>
  machineSetNodes.value.find((n) => n.metadata.id === machineId),
)

const isControlPlane = computed(
  () => typeof clusterMachineStatus.value?.metadata.labels?.[LabelControlPlaneRole] === 'string',
)

const controlPlaneCount = computed(
  () =>
    machineSetNodes.value?.filter(
      (n) => n.metadata.labels?.[LabelMachineSet] === controlPlaneMachineSetId(clusterId),
    ).length ?? 0,
)

const warning = computed(() => {
  const cpCount = controlPlaneCount.value

  if (!isControlPlane.value || cpCount === 1 || cpCount % 2 === 0) return null

  return `${pluralize('Control Plane', cpCount - 1, true)} will not provide fault-tolerance with etcd quorum requirements. Removing this machine will result in an even number of control plane nodes, which reduces the fault tolerance of the etcd cluster.`
})

const node = computed(
  () =>
    clusterMachineStatus.value?.metadata.labels?.[ClusterMachineStatusLabelNodeName] || machineId,
)

const destroyNode = async () => {
  isDestroying.value = true

  try {
    const resources: DeleteRequest[] = []
    const links: DeleteRequest[] = []

    if (isControlPlane.value && controlPlaneCount.value === 1) {
      throw new ClusterCommandError(
        `Failed to Destroy Node ${machineId}`,
        'The cluster should have at least one control plane running',
      )
    }

    if (forceEtcdLeave.value) {
      // if this is a force-delete, MachineSetNode might already be gone, and it is ok
      try {
        await ResourceService.Get<Resource<NodeForceDestroyRequestSpec>>(
          {
            type: NodeForceDestroyRequestType,
            namespace: DefaultNamespace,
            id: machineId,
          },
          withRuntime(Runtime.Omni),
        )
      } catch (e) {
        if (!(e instanceof RequestError) || e.code !== Code.NOT_FOUND) {
          throw e
        }

        await ResourceService.Create<Resource<NodeForceDestroyRequestSpec>>(
          {
            metadata: {
              id: machineId,
              namespace: DefaultNamespace,
              type: NodeForceDestroyRequestType,
            },
            spec: {},
          },
          withRuntime(Runtime.Omni),
        )
      }
    } else if (!forceDestroy.value && !currentMachineSetNode.value) {
      // if this is not a force-delete, and the node is not found, it is an error
      throw new ClusterCommandError(
        `Failed to Destroy Node ${machineId}`,
        'The node with such id is not part of the cluster',
      )
    }

    if (currentMachineSetNode.value) {
      const { id, namespace, type } = currentMachineSetNode.value.metadata
      resources.push({ id, namespace, type })
    }

    if (forceDestroy.value) {
      links.push({
        id: machineId,
        namespace: DefaultNamespace,
        type: SiderolinkResourceType,
      })
    }

    await teardownResources(links)
    await destroyResources(resources)

    const messages = ['The machine set is scaling down.']
    if (forceEtcdLeave.value) messages.push('Force leave etcd was requested.')
    if (forceDestroy.value) messages.push('Force destroy was requested.')

    showSuccess(`The Machine ${node.value} was Removed From the Machine Set`, messages.join(' '))

    emit('onDestroy')
    open.value = false
  } catch (e) {
    if (e instanceof ClusterCommandError) {
      showError(e.errorNotification.title, e.errorNotification.details)
    } else {
      showError('Failed to Destroy The Node', e instanceof Error ? e.message : String(e))
    }
  } finally {
    isDestroying.value = false
  }
}
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    title="Destroy machine"
    action-label="Destroy"
    :disabled="!!clusterMachineStatusErr || !!machineSetNodesErr"
    :loading="clusterMachineStatusLoading || machineSetNodesLoading || isDestroying"
    @confirm="destroyNode"
  >
    <template #description>Node {{ node }}</template>

    <div class="max-w-sm">
      <ManagedByTemplatesWarning warning-style="popup" />

      <div class="flex flex-col gap-2">
        <p v-if="clusterMachineStatusErr || machineSetNodesErr" class="text-xs text-red-r1">
          <span v-if="clusterMachineStatusErr">{{ clusterMachineStatusErr }}</span>
          <span v-if="machineSetNodesErr">{{ machineSetNodesErr }}</span>
        </p>

        <Tooltip>
          <template #description>
            <p class="w-96">
              Force leaving etcd member may break the etcd quorum. Do not use this option unless you
              are sure that the machine is not an etcd member or the machine is already down and
              will not come back up.
            </p>
          </template>

          <TCheckbox
            v-model="forceEtcdLeave"
            :disabled="!isControlPlane"
            label="force etcd leave"
          />
        </Tooltip>

        <Tooltip>
          <template #description>
            <p class="w-96">
              Force destroying the machine will skip wiping the machine and will remove it from the
              Omni account completely. It will be necessary to manually wipe the machine to add it
              to the Omni account again. Use this option only in case if the machine is already down
              and will not come back up.
            </p>
          </template>

          <TCheckbox v-model="forceDestroy" label="force destroy" />
        </Tooltip>
      </div>

      <div v-if="warning" class="mt-3 text-xs text-yellow-y1">{{ warning }}</div>

      <p class="my-2 text-xs">Please confirm the action.</p>
    </div>
  </ConfirmModal>
</template>
