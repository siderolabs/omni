<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import type { StatusAppearance } from '@/components/Status/StatusPill.vue'
import StatusPill from '@/components/Status/StatusPill.vue'
import { NodesViewFilterOptions, TCommonStatuses, TPodsViewFilterOptions } from '@/constants'

type Props = {
  title: string
}

const { title } = defineProps<Props>()

const status = computed<StatusAppearance>(() => {
  switch (title) {
    case TPodsViewFilterOptions.RUNNING:
    case TPodsViewFilterOptions.SUCCEEDED:
    case NodesViewFilterOptions.READY:
    case TCommonStatuses.ACTIVE:
    case TCommonStatuses.COMPLETED:
    case TCommonStatuses.HEALTHY:
    case TCommonStatuses.ENABLED:
    case TCommonStatuses.ON:
    case TCommonStatuses.UP_TO_DATE:
    case TCommonStatuses.APPLIED:
      return { tone: 'success', glyph: 'success' }
    case TCommonStatuses.PENDING:
    case TCommonStatuses.PROVISIONED:
    case TCommonStatuses.PROVISIONING:
      return { tone: 'warning', glyph: 'progress' }
    case TCommonStatuses.FAILED:
    case TCommonStatuses.ERROR:
    case TCommonStatuses.PROVISION_FAILED:
    case TCommonStatuses.UNHEALTHY:
    case TCommonStatuses.OUTDATED:
    case NodesViewFilterOptions.NOT_READY:
      return { tone: 'danger', glyph: 'danger' }
    case TCommonStatuses.DISCONNECTED:
      return { tone: 'danger', glyph: 'warning' }
    case TCommonStatuses.DEPROVISIONING:
      return { tone: 'info', glyph: 'progress' }
    case TCommonStatuses.INITIALIZED:
    case TCommonStatuses.PREPARING:
    case TCommonStatuses.STARTING:
    case TCommonStatuses.STOPPING:
    case TCommonStatuses.WAITING:
    case TCommonStatuses.LOADING:
      return { tone: 'info', glyph: 'progress' }
    case TCommonStatuses.FINISHED:
    case TCommonStatuses.SKIPPED:
    case TCommonStatuses.EXPIRED:
    case TCommonStatuses.DISABLED:
    case TCommonStatuses.OFF:
      return { tone: 'info', glyph: 'neutral' }
    case TCommonStatuses.REVOKED:
    case TCommonStatuses.FALSE:
      return { tone: 'info', glyph: 'danger' }
    case TCommonStatuses.TRUE:
      return { tone: 'info', glyph: 'success' }
    default:
      return { tone: 'info', glyph: 'unknown' }
  }
})
</script>

<template>
  <StatusPill :tone="status.tone" :glyph="status.glyph">
    {{ title }}
  </StatusPill>
</template>
