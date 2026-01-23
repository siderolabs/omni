<!--
Copyright (c) 2026 Sidero Labs, Inc.

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
import OverviewRightPanelItem from '@/views/cluster/Overview/components/OverviewRightPanel/OverviewRightPanelItem.vue'

const mapping: Record<ConditionType | number, string> = {}
for (const key of Object.keys(ConditionType)) {
  mapping[ConditionType[key as keyof typeof ConditionType]] = key
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
      return 'var(--color-yellow-y1)'
    case ControlPlaneStatusSpecConditionSeverity.Error:
      return 'var(--color-red-r1)'
  }

  return 'var(--color-naturals-n12)'
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
