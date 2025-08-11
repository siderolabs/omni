<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, toRefs } from 'vue'

import type { Resource } from '@/api/grpc'
import type { ClusterMachineRequestStatusSpec } from '@/api/omni/specs/omni.pb'
import { ClusterMachineRequestStatusSpecStage } from '@/api/omni/specs/omni.pb'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TStatus from '@/components/common/Status/TStatus.vue'
import IconHeaderDropdownLoading from '@/components/icons/IconHeaderDropdownLoading.vue'
import { TCommonStatuses } from '@/constants'

const props = defineProps<{
  requestStatus: Resource<ClusterMachineRequestStatusSpec>
}>()

const { requestStatus } = toRefs(props)

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
</script>

<template>
  <div class="flex h-3 items-center gap-1 py-6 pr-11 pl-3 text-xs text-naturals-n14">
    <div class="pointer-events-none w-5" />
    <div class="-mr-3 grid flex-1 grid-cols-4 items-center">
      <div class="col-span-2 flex items-center gap-2">
        <IconHeaderDropdownLoading
          v-if="stage !== TCommonStatuses.PROVISIONED"
          active
          class="ml-2 h-4 w-4"
        />
        <TIcon v-else icon="cloud-connection" class="ml-2 h-4 w-4" />
        {{ requestStatus.metadata.id }}
      </div>
      <div>
        <TStatus :title="stage" />
      </div>
      <div class="text-naturals-N11 truncate text-xs" :title="requestStatus.spec.status">
        {{ requestStatus.spec.status }}
      </div>
    </div>
  </div>
</template>
