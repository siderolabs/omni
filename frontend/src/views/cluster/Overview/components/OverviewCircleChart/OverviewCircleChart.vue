<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { ApexOptions } from 'apexcharts'
import { computed, toRefs } from 'vue'
import ApexChart from 'vue3-apexcharts'

const props = defineProps<{ chartFillPercents: number | string }>()

const { chartFillPercents } = toRefs(props)
const options: ApexOptions = {
  chart: {
    dropShadow: {
      enabled: true,
      top: 0,
      left: 0,
      blur: 5,
      color: 'var(--color-primary-p3)',
      opacity: 0.2,
    },
  },
  plotOptions: {
    radialBar: {
      hollow: {
        margin: 0,
        size: '60',
      },
      track: {
        background: 'var(--color-naturals-n0)',
      },
      dataLabels: {
        show: false,
      },
    },
  },
  fill: {
    colors: ['var(--color-primary-p3)'],
  },
  stroke: {
    lineCap: 'round',
  },
  states: {
    hover: {
      filter: {
        type: 'none',
      },
    },
    active: {
      filter: {
        type: 'none',
      },
    },
  },
}

const percentage = computed(() => chartFillPercents.value ?? 0)
</script>

<template>
  <div class="z-0 flex items-center justify-start">
    <ApexChart
      :height="200"
      :width="200"
      type="radialBar"
      :options="options"
      :series="[percentage]"
    />
  </div>
</template>
