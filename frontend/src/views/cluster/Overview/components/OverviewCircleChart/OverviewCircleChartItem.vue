<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="item">
    <h4 class="item__name">{{ name }}</h4>
    <overview-circle-chart :chartFillPercents="chartFillPercents" />
    <p class="item__use">
      {{ usageName }}
      <span class="item__use-percents">{{ usagePercents }}%</span>
    </p>
    <p class="item__total">{{ usageTotal }} {{ totalLegend }}</p>
  </div>
</template>

<script setup lang="ts">
import OverviewCircleChart from "@/views/cluster/Overview/components/OverviewCircleChart/OverviewCircleChart.vue";
import { toRefs } from "vue";

type Props = {
  name: string;
  usageName: string;
  totalLegend?: string;
  usagePercents: number | string;
  usageTotal: number | string;
  chartFillPercents: number | string;
};

const props = withDefaults(defineProps<Props>(), {
  totalLegend: "Total",
});

const { name, usageName, totalLegend, usagePercents, usageTotal, chartFillPercents } = toRefs(props);
</script>

<style scoped>
.item {
  @apply flex flex-col items-center justify-start mb-10 2xl:mb-0;
}
.item__name {
  @apply text-xs font-medium text-naturals-N13;
}
.item__use {
  @apply text-xs font-medium text-naturals-N13 mb-4;
}
.item__use-percents {
  @apply text-xs font-medium text-naturals-N10 pl-2;
}
.item__total {
  @apply text-xs font-medium text-naturals-N10;
}
</style>
