<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { ApexOptions } from 'apexcharts'
import { computed, defineAsyncComponent } from 'vue'

const ApexChart = defineAsyncComponent(() => import('vue3-apexcharts'))

interface Props {
  title: string
  showHollowTotal?: boolean
  vertical?: boolean
  total?: number
  items: {
    label: string
    value: number
  }[]
  legendFormatter?: (value: number) => string
}

const props = withDefaults(defineProps<Props>(), {
  total: undefined,
  legendFormatter: (value: number) => value.toString(),
})

const total = computed(
  () => props.total ?? props.items.reduce((prev, curr) => prev + curr.value, 0),
)

const series = computed(() =>
  props.total === 0
    ? Array<number>(props.items.length).fill(0)
    : props.items.map((i) => Math.round((i.value / total.value) * 100)),
)

const legendItems = computed(() => [
  {
    label: 'Total',
    value: props.legendFormatter(total.value),
    color: 'var(--color-naturals-n8)',
  },
  ...props.items.map((item, i) => ({
    label: item.label,
    value: props.legendFormatter(item.value),
    color: colors[i],
  })),
])

const colors = [
  'var(--color-primary-p3)',
  'var(--color-red-r1)',
  'var(--color-green-g1)',
  'var(--color-blue-b1)',
  'var(--color-yellow-y1)',
]

const options = computed<ApexOptions>(() => ({
  plotOptions: {
    radialBar: {
      hollow: {
        size: `${80 - props.items.length * 10}`,
      },
      track: {
        margin: 2,
        background: [
          'var(--color-naturals-n8)',
          ...Array<string>(props.items.length - 1).fill('transparent'),
        ],
      },
      dataLabels: {
        name: { show: false },
        total: {
          show: props.showHollowTotal,
          formatter: () =>
            props.legendFormatter(total.value) /* To prevent library's calculation */,
        },
        value: {
          show: props.showHollowTotal,
          offsetY: 5,
          color: 'var(--color-naturals-n14)',
          fontSize: 'var(--text-base)',
          fontWeight: 'var(--font-weight-medium)',
          formatter: () => props.legendFormatter(total.value), // To always show total instead of individual values
        },
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
}))
</script>

<template>
  <div class="flex flex-col gap-2">
    <h2 class="text-xl font-medium text-naturals-n14">{{ title }}</h2>

    <figure
      class="flex items-center gap-2 self-center py-2 not-visited:px-4"
      :class="{ 'flex-col': vertical }"
    >
      <ApexChart type="radialBar" width="200" :options="options" :series="series" />

      <figcaption class="flex flex-col gap-2">
        <dl
          v-for="item in legendItems"
          :key="item.label"
          class="flex items-center gap-2 text-xs whitespace-nowrap"
        >
          <span
            aria-hidden="true"
            class="size-2 rounded-xs"
            :style="{ backgroundColor: item.color }"
          />
          <dt class="text-naturals-n11">{{ item.label }}</dt>
          <dd class="text-naturals-n14">{{ item.value }}</dd>
        </dl>
      </figcaption>
    </figure>
  </div>
</template>
