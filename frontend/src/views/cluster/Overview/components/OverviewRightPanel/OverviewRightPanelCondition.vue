<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <overview-right-panel-item
    :name="getConditionName(condition?.type)">
      <tooltip :description="condition.reason" placement="left">
        <span v-bind:class="condition.status === ControlPlaneStatusSpecConditionStatus.Ready ? '' : 'text-red-R1'">
          {{ condition.status === ControlPlaneStatusSpecConditionStatus.Ready ? 'OK' : 'Not Ready' }}
        </span>
      </tooltip>
  </overview-right-panel-item>
</template>

<script setup lang="ts">
import {
ConditionType,
  ControlPlaneStatusSpecCondition,
  ControlPlaneStatusSpecConditionStatus,
} from "@/api/omni/specs/omni.pb";

import OverviewRightPanelItem from "@/views/cluster/Overview/components/OverviewRightPanel/OverviewRightPanelItem.vue";
import Tooltip from "@/components/common/Tooltip/Tooltip.vue";

const mapping: Record<ConditionType | number, string> = {};
for (const key of Object.keys(ConditionType)) {
  mapping[ConditionType[key]] = key;
}

const getConditionName = (t?: ConditionType) => {
  if (t === undefined) {
    return "Unknown condition";
  }

  if (t === ConditionType.WireguardConnection) {
    return "WireGuard Connection";
  }

  return mapping[t];
}

type Props = {
  condition: ControlPlaneStatusSpecCondition;
};

defineProps<Props>();
</script>
