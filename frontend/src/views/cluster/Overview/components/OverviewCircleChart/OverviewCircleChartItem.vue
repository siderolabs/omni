<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { toRefs } from 'vue'

import OverviewCircleChart from '@/views/cluster/Overview/components/OverviewCircleChart/OverviewCircleChart.vue'

type Props = {
  name: string
  usageName: string
  totalLegend?: string
  usagePercents: number | string
  usageTotal: number | string
  chartFillPercents: number | string
}

const props = withDefaults(defineProps<Props>(), {
  totalLegend: 'Total',
})

const { name, usageName, totalLegend, usagePercents, usageTotal, chartFillPercents } = toRefs(props)
</script>

<template>
  <div class="item">
    <h4 class="item__name">{{ name }}</h4>
    <OverviewCircleChart :chart-fill-percents="chartFillPercents" />
    <p class="item__use">
      {{ usageName }}
      <span class="item__use-percents">{{ usagePercents }}%</span>
    </p>
    <p class="item__total">{{ usageTotal }} {{ totalLegend }}</p>
  </div>
</template>

<style scoped>
@reference "../../../../../index.css";

.item {
  @apply mb-10 flex flex-col items-center justify-start 2xl:mb-0;
}
.item__name {
  @apply text-xs font-medium text-naturals-n13;
}
.item__use {
  @apply mb-4 text-xs font-medium text-naturals-n13;
}
.item__use-percents {
  @apply pl-2 text-xs font-medium text-naturals-n10;
}
.item__total {
  @apply text-xs font-medium text-naturals-n10;
}
</style>
