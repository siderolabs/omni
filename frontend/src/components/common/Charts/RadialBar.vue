<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="text-xs font-medium flex flex-col items-center justify-center gap-1">
    <h4 class="text-naturals-N13">{{ name }}</h4>
    <apex-chart
      class="flex-1"
      :height="200"
      :width="200"
      type="radialBar"
      :options="options"
      :series="normalized"
    />
    <div class="flex flex-col gap-1 -mt-4 w-full px-4">
      <div
        v-for="(label, index) in labels"
        :key="label"
        class="flex gap-1 items-center text-naturals-N13"
      >
        <div class="w-2.5 h-2.5 rounded-sm" :style="{ 'background-color': colors[index] }" />
        <div class="flex-1">{{ label }}</div>
        <div>{{ formatter(series[index]) }}</div>
      </div>
      <div class="flex gap-1 items-center">
        <div class="flex-1">Capacity</div>
        <div>{{ formatter(total) }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { primary, naturals, green, red, yellow } from '@/vars/colors'
import type { ApexOptions } from 'apexcharts'
import type { Ref } from 'vue'
import { computed, toRefs } from 'vue'

import ApexChart from 'vue3-apexcharts'

type Props = {
  name: string
  labels?: string[]
  total: number
  series: number[]

  formatter?: (input: number) => string
}

const props = withDefaults(defineProps<Props>(), {
  formatter: (input: number) => input.toFixed(2),
})

const { total, series } = toRefs(props)

const normalized = computed(() => {
  return series.value.map((value) => {
    if (!total.value) {
      return 0
    }

    return (value / total.value) * 100
  })
})

const colors = [primary.P3, red.R1, green.G1, yellow.Y1]

const options: Ref<ApexOptions> = computed(() => {
  return {
    chart: {},
    yaxis: {
      max: total?.value ?? 0,
    },
    plotOptions: {
      radialBar: {
        hollow: {
          size: `${80 - series.value.length * 20}`,
        },
        track: {
          background: naturals.N0,
        },
        dataLabels: {
          show: false,
        },
      },
    },
    fill: {
      colors: colors,
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
})
</script>
