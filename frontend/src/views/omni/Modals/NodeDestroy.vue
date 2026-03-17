<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import type { Ref } from 'vue'
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { Code } from '@/api/google/rpc/code.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { ClusterMachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime, withSelectors } from '@/api/options'
import {
  ClusterMachineStatusType,
  DefaultNamespace,
  LabelCluster,
  LabelControlPlaneRole,
  LabelMachineSet,
  MachineSetNodeType,
} from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import { ClusterCommandError, destroyNodes } from '@/methods/cluster'
import { controlPlaneMachineSetId } from '@/methods/machineset'
import { setupNodenameWatch } from '@/methods/node'
import { showError, showSuccess } from '@/notification'
import ManagedByTemplatesWarning from '@/views/cluster/ManagedByTemplatesWarning.vue'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const router = useRouter()
const route = useRoute()
const scalingDown = ref(false)
const warning: Ref<string | null> = ref(null)

let closed = false

const close = (goBack?: boolean) => {
  if (closed) {
    return
  }

  if (!goBack && !route.query.goback) {
    router.push({ name: 'ClusterOverview', params: { cluster: route.query.cluster as string } })

    return
  }

  closed = true

  router.go(-1)
}

const canForceEtcdLeave = ref(false)
const forceEtcdLeave = ref(false)
const force = ref(false)

const initRequiresForceEtcdLeave = async () => {
  try {
    const m = await ResourceService.Get(
      {
        type: ClusterMachineStatusType,
        namespace: DefaultNamespace,
        id: route.query.machine as string,
      },
      withRuntime(Runtime.Omni),
    )

    if (m.metadata.labels?.[LabelControlPlaneRole] !== undefined) {
      canForceEtcdLeave.value = true
    }
  } catch (e) {
    if (e.code !== Code.NOT_FOUND) {
      throw e
    }
  }
}

onMounted(async () => {
  let controlPlanes = 0
  let isControlPlane = true

  try {
    await initRequiresForceEtcdLeave()

    const clusterMachineStatus: Resource<ClusterMachineStatusSpec> = await ResourceService.Get(
      {
        type: ClusterMachineStatusType,
        namespace: DefaultNamespace,
        id: route.query.machine as string,
      },
      withRuntime(Runtime.Omni),
    )

    isControlPlane = clusterMachineStatus?.metadata?.labels?.[LabelControlPlaneRole] === ''

    // since this code called for both worker and control plane nodes, we need to check if the node is control plane node
    const controlPlaneMachineStatus: Resource<ClusterMachineStatusSpec> = await ResourceService.Get(
      {
        type: ClusterMachineStatusType,
        namespace: DefaultNamespace,
        id: route.query.machine as string,
      },
      withRuntime(Runtime.Omni),
      withSelectors([
        `${LabelCluster}=${route.query.cluster as string}`,
        `${LabelControlPlaneRole}`,
      ]),
    )

    if (!controlPlaneMachineStatus) {
      warning.value = null
      return
    }

    const resp = await ResourceService.List(
      {
        namespace: DefaultNamespace,
        type: MachineSetNodeType,
      },
      withRuntime(Runtime.Omni),
      withSelectors([
        `${LabelMachineSet}=${controlPlaneMachineSetId('cluster' in route.params ? route.params.cluster : '')}`,
      ]),
    )

    controlPlanes = resp.length
  } catch (e) {
    close(true)

    showError('Failed to Destroy The Node', e.message)

    return
  }

  if (!isControlPlane || controlPlanes % 2 === 0 || controlPlanes === 1) {
    warning.value = null

    return
  }

  warning.value = `${pluralize('Control Plane', controlPlanes - 1, true)} will not provide fault-tolerance with etcd quorum requirements. Removing this machine will result in an even number of control plane nodes, which reduces the fault tolerance of the etcd cluster.`
})

const node = setupNodenameWatch(route.query.machine as string)

const destroyNode = async () => {
  scalingDown.value = true

  if (!route.query.machine) {
    showError('Failed to Destroy The Machine', 'The machine id not resolved')

    close(true)

    return
  }

  try {
    await destroyNodes(
      route.query.cluster as string,
      [route.query.machine as string],
      forceEtcdLeave.value,
      force.value,
    )
  } catch (e) {
    if (e instanceof ClusterCommandError) {
      showError(e.errorNotification.title, e.errorNotification.details)

      close(true)

      return
    }

    close(true)

    showError('Failed to Destroy The Node', e.message)

    return
  } finally {
    scalingDown.value = false
  }

  close()

  let message = 'The machine set is scaling down.'
  if (forceEtcdLeave.value) {
    message += ' Force leave etcd was requested.'
  }

  if (force.value) {
    message += ' Force destroy was requested.'
  }

  showSuccess(`The Machine ${node.value} was Removed From the Machine Set`, message)
}
</script>

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">
        Destroy the Node {{ node ?? $route.query.machine }} ?
      </h3>
      <CloseButton @click="close(true)" />
    </div>
    <ManagedByTemplatesWarning warning-style="popup" />

    <div class="flex flex-col gap-2">
      <Tooltip>
        <template #description>
          <p class="w-96">
            Force leaving etcd member may break the etcd quorum. Do not use this option unless you
            are sure that the machine is not an etcd member or the machine is already down and will
            not come back up.
          </p>
        </template>
        <TCheckbox
          v-model="forceEtcdLeave"
          :disabled="!canForceEtcdLeave"
          label="force etcd leave"
        />
      </Tooltip>
      <Tooltip>
        <template #description>
          <p class="w-96">
            Force destroying the machine will skip wiping the machine and will remove it from the
            Omni account completely. It will be necessary to manually wipe the machine to add it to
            the Omni account again. Use this option only in case if the machine is already down and
            will not come back up.
          </p>
        </template>
        <TCheckbox v-model="force" label="force destroy" />
      </Tooltip>
    </div>

    <div v-if="warning" class="mt-3 text-xs text-yellow-y1">{{ warning }}</div>

    <p class="my-2 text-xs">Please confirm the action.</p>

    <div class="mt-2 flex items-end gap-4">
      <div class="flex-1" />
      <TButton class="h-9 w-32" :disabled="scalingDown" @click="destroyNode">
        <TSpinner v-if="scalingDown" class="h-5 w-5" />
        Confirm
      </TButton>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-n14;
}
</style>
