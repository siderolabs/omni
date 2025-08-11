<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, toRefs } from 'vue'

import type { ControlPlaneStatusSpecCondition } from '@/api/omni/specs/omni.pb'
import {
  ConditionType,
  ControlPlaneStatusSpecConditionSeverity,
  ControlPlaneStatusSpecConditionStatus,
} from '@/api/omni/specs/omni.pb'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import { naturals, red, yellow } from '@/vars/colors'
import OverviewRightPanelItem from '@/views/cluster/Overview/components/OverviewRightPanel/OverviewRightPanelItem.vue'

const mapping: Record<ConditionType | number, string> = {}
for (const key of Object.keys(ConditionType)) {
  mapping[ConditionType[key]] = key
}

const getConditionName = (t?: ConditionType) => {
  if (t === undefined) {
    return 'Unknown condition'
  }

  if (t === ConditionType.WireguardConnection) {
    return 'WireGuard Connection'
  }

  return mapping[t]
}

type Props = {
  condition: ControlPlaneStatusSpecCondition
}

const props = defineProps<Props>()

const { condition } = toRefs(props)

const color = computed(() => {
  switch (condition.value.severity) {
    case ControlPlaneStatusSpecConditionSeverity.Warning:
      return yellow.Y1
    case ControlPlaneStatusSpecConditionSeverity.Error:
      return red.R1
  }

  return naturals.N12
})

const text = computed(() => {
  if (condition.value.status === ControlPlaneStatusSpecConditionStatus.Ready) {
    return 'OK'
  }

  switch (condition.value.severity) {
    case ControlPlaneStatusSpecConditionSeverity.Warning:
      return 'Warning'
    case ControlPlaneStatusSpecConditionSeverity.Error:
      return 'Not Ready'
  }

  return 'Unknown'
})
</script>

<template>
  <OverviewRightPanelItem :name="getConditionName(condition?.type)">
    <Tooltip :description="condition.reason" placement="left">
      <span :style="{ color: color }">
        {{ text }}
      </span>
    </Tooltip>
  </OverviewRightPanelItem>
</template>
