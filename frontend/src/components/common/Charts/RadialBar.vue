<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
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

const colors = [
  'var(--color-primary-p3)',
  'var(--color-red-r1)',
  'var(--color-green-g1)',
  'var(--color-yelow-y1)',
]

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
          background: 'var(--color-naturals-n0)',
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

<template>
  <div class="flex flex-col items-center justify-center gap-1 text-xs font-medium">
    <h4 class="text-naturals-n13">{{ name }}</h4>
    <ApexChart
      class="flex-1"
      :height="200"
      :width="200"
      type="radialBar"
      :options="options"
      :series="normalized"
    />
    <div class="-mt-4 flex w-full flex-col gap-1 px-4">
      <div
        v-for="(label, index) in labels"
        :key="label"
        class="flex items-center gap-1 text-naturals-n13"
      >
        <div class="h-2.5 w-2.5 rounded-sm" :style="{ 'background-color': colors[index] }" />
        <div class="flex-1">{{ label }}</div>
        <div>{{ formatter(series[index]) }}</div>
      </div>
      <div class="flex items-center gap-1">
        <div class="flex-1">Capacity</div>
        <div>{{ formatter(total) }}</div>
      </div>
    </div>
  </div>
</template>
