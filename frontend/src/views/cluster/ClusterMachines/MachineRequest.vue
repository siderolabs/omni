<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex items-center h-3 gap-1 text-xs py-6 pl-3 pr-11 text-naturals-N14 bg-center bg-no-repeat bg-opacity-25">
    <div class="w-5 pointer-events-none"/>
    <div class="flex-1 grid grid-cols-4 -mr-3 items-center">
      <div class="col-span-2 flex items-center gap-2">
        <icon-header-dropdown-loading v-if="stage !== TCommonStatuses.PROVISIONED" active class="w-4 h-4 ml-2"/>
        <t-icon v-else icon="cloud-connection" class="w-4 h-4 ml-2"/>
        {{ requestStatus.metadata.id }}
      </div>
      <div>
        <t-status :title="stage"/>
      </div>
      <div class="truncate text-xs text-naturals-N11" :title="requestStatus.spec.status">
        {{ requestStatus.spec.status }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Resource } from "@/api/grpc";

import TStatus from "@/components/common/Status/TStatus.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";
import { ClusterMachineRequestStatusSpec, ClusterMachineRequestStatusSpecStage } from "@/api/omni/specs/omni.pb";
import { computed, toRefs } from "vue";
import { TCommonStatuses } from "@/constants";
import IconHeaderDropdownLoading from "@/components/icons/IconHeaderDropdownLoading.vue";

const props = defineProps<{
  requestStatus: Resource<ClusterMachineRequestStatusSpec>,
}>();

const { requestStatus } = toRefs(props);

const stage = computed(() => {
  switch (requestStatus.value.spec.stage) {
  case ClusterMachineRequestStatusSpecStage.FAILED:
    return TCommonStatuses.PROVISION_FAILED;
  case ClusterMachineRequestStatusSpecStage.PROVISIONING:
    return TCommonStatuses.PROVISIONING;
  case ClusterMachineRequestStatusSpecStage.PENDING:
    return TCommonStatuses.PENDING;
  case ClusterMachineRequestStatusSpecStage.PROVISIONED:
    return TCommonStatuses.PROVISIONED;
  case ClusterMachineRequestStatusSpecStage.DEPROVISIONING:
    return TCommonStatuses.DEPROVISIONING;
  }

  return TCommonStatuses.UNKNOWN;
})
</script>
