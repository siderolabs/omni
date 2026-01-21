<!--
Copyright (c) 2026 Sidero Labs, Inc.

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
  <div
    class="col-span-full grid grid-cols-subgrid items-center py-3 pr-4 pl-2 text-xs text-naturals-n14"
  >
    <div class="col-span-2 ml-6 flex items-center gap-2">
      <IconHeaderDropdownLoading
        v-if="stage !== TCommonStatuses.PROVISIONED"
        active
        class="size-4"
      />
      <TIcon v-else icon="cloud-connection" class="size-4" />
      {{ requestStatus.metadata.id }}
    </div>

    <TStatus :title="stage" />

    <div class="text-naturals-N11 truncate text-xs" :title="requestStatus.spec.status">
      {{ requestStatus.spec.status }}
    </div>
  </div>
</template>
