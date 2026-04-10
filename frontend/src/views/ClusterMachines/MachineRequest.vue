<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, toRefs } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { ClusterMachineRequestStatusSpec } from '@/api/omni/specs/omni.pb'
import { ClusterMachineRequestStatusSpecStage } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import { InfraProviderNamespace, MachineRequestType } from '@/api/resources'
import TActionsBox from '@/components/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/ActionsBox/TActionsBoxItem.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import IconHeaderDropdownLoading from '@/components/icons/IconHeaderDropdownLoading.vue'
import TStatus from '@/components/Status/TStatus.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { TCommonStatuses } from '@/constants'
import { showError, showSuccess } from '@/notification'

const props = defineProps<{
  requestStatus: Resource<ClusterMachineRequestStatusSpec>
  canDestroy: boolean
}>()

const { requestStatus } = toRefs(props)
const destroying = ref(false)

const stage = computed(() => {
  switch (requestStatus.value.spec.stage) {
    case ClusterMachineRequestStatusSpecStage.FAILED:
      return TCommonStatuses.PROVISION_FAILED
    case ClusterMachineRequestStatusSpecStage.PROVISIONING:
      return TCommonStatuses.PROVISIONING
    case ClusterMachineRequestStatusSpecStage.PENDING:
      return TCommonStatuses.PENDING
    case ClusterMachineRequestStatusSpecStage.PROVISIONED:
      return TCommonStatuses.PROVISIONED
    case ClusterMachineRequestStatusSpecStage.DEPROVISIONING:
      return TCommonStatuses.DEPROVISIONING
  }

  return TCommonStatuses.UNKNOWN
})

const isDestroyableStage = computed(() => {
  return (
    stage.value === TCommonStatuses.PROVISION_FAILED ||
    stage.value === TCommonStatuses.PROVISIONING ||
    stage.value === TCommonStatuses.PROVISIONED ||
    stage.value === TCommonStatuses.PENDING
  )
})

const destroyDisabled = computed(() => {
  if (!props.canDestroy) return 'Insufficient permissions to destroy machine requests'
  if (!isDestroyableStage.value) return 'Cannot destroy a machine request in this stage'

  return undefined
})

const forceDestroy = async () => {
  const id = requestStatus.value.metadata.id!

  destroying.value = true

  try {
    await ResourceService.Teardown(
      {
        type: MachineRequestType,
        namespace: InfraProviderNamespace,
        id: id,
      },
      withRuntime(Runtime.Omni),
    )

    showSuccess('Machine Request Destroyed', `Machine request ${id} is being torn down.`)
  } catch (e) {
    showError('Failed to Destroy Machine Request', (e as Error).message)
  } finally {
    destroying.value = false
  }
}
</script>

<template>
  <div
    class="col-span-full grid grid-cols-subgrid items-center py-3 pr-4 pl-2 text-xs text-naturals-n14"
  >
    <div class="col-span-2 ml-6 flex items-center gap-2">
      <IconHeaderDropdownLoading
        v-if="stage !== TCommonStatuses.PROVISIONED"
        active
        class="size-4 shrink-0"
      />
      <TIcon v-else icon="cloud-connection" class="size-4 shrink-0" />
      {{ requestStatus.metadata.id }}
    </div>

    <TStatus :title="stage" />

    <div class="truncate text-xs" :title="requestStatus.spec.status">
      {{ requestStatus.spec.status }}
    </div>

    <div class="flex items-center justify-end">
      <TActionsBox>
        <Tooltip :description="destroyDisabled" placement="left">
          <TActionsBoxItem
            :icon="destroying ? 'loading' : 'delete'"
            danger
            :disabled="!!destroyDisabled || destroying"
            @select="forceDestroy"
          >
            {{ destroying ? 'Destroying...' : 'Destroy' }}
          </TActionsBoxItem>
        </Tooltip>
      </TActionsBox>
    </div>
  </div>
</template>
