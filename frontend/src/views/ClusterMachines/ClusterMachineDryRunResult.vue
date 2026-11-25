<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import type { RouteLocationRaw } from 'vue-router'

import { type DryRunStatusSpec, DryRunStatusSpecOutcome } from '@/api/omni/specs/omni.pb'
import StatusGlyph, { type StatusGlyphType } from '@/components/Status/StatusGlyph.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'

import ClusterMachineDryRunDescription from './ClusterMachineDryRunDescription.vue'
import { dryRunResult } from './dryRunSummary'

const { dryRun, standalone, to } = defineProps<{
  dryRun: DryRunStatusSpec
  standalone?: boolean
  to: RouteLocationRaw
}>()

const result = computed(() => dryRunResult(dryRun))

const glyph = computed((): StatusGlyphType => {
  switch (dryRun.outcome) {
    case DryRunStatusSpecOutcome.PASSED:
      return 'success'
    // the dry run changed nothing on the machine, so it needs attention rather than being a failure
    case DryRunStatusSpecOutcome.FAILED:
      return 'warning'
    default:
      return 'unknown'
  }
})

const tone = computed(() => {
  switch (glyph.value) {
    case 'success':
      return 'text-status-success-default'
    case 'warning':
      return 'text-status-warning-default'
    default:
      return 'text-status-info-default'
  }
})

const detailed = computed(
  () => !!dryRun.error || dryRun.outcome === DryRunStatusSpecOutcome.NOT_CHECKED,
)
</script>

<template>
  <Tooltip v-if="result" :disabled="!detailed" keep-open>
    <RouterLink
      :to
      class="flex shrink-0 items-center gap-micro rounded text-content-default hover:text-content-emphasis"
      data-testid="cluster-machine-dry-run"
      @click.stop
    >
      <span v-if="!standalone" class="text-content-muted" aria-hidden="true">·</span>
      <StatusGlyph
        :glyph="glyph"
        :class="tone"
        aria-hidden="true"
        :data-testid="`cluster-machine-dry-run-${glyph}`"
      />
      {{ standalone ? `Pending config ${result}` : result }}
    </RouterLink>

    <template #description>
      <ClusterMachineDryRunDescription :dry-run="dryRun" />
    </template>
  </Tooltip>
</template>
