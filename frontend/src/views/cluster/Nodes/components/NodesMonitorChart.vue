<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col">
    <div class="flex justify-between">
      <div v-if="title" class="w-full text-left text-naturals-N13 pl-3 text-xs">
        {{ title }}
      </div>
      <div v-if="total" class="w-full text-right pr-3 text-xs">
        {{ total }}
      </div>
    </div>
    <div id="chartContainer" class="flex-1">
      <div v-if="err || loading" class="flex flex-row justify-center items-center w-full h-full">
        <div
          v-if="err"
          class="flex justify-center items-center w-1/2 gap-4 text-talos-gray-500 text-sm"
        >
          <div class="flex-0">
            <exclamation-circle-icon class="w-6 h-6" />
          </div>
          <div>{{ err }}</div>
        </div>
        <t-spinner v-else class="w-5 h-5" />
      </div>
      <apex-chart
        v-else
        width="100%"
        height="100%"
        :type="type"
        :options="options"
        :series="series ?? []"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Ref } from 'vue'
import { ref, toRefs, computed } from 'vue'

import type { WatchContext } from '@/api/watch'
import ApexChart from 'vue3-apexcharts'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import { ExclamationCircleIcon } from '@heroicons/vue/24/outline'
import { DateTime } from 'luxon'
import type { WatchResponse } from '@/api/omni/resources/resources.pb'
import { EventType } from '@/api/omni/resources/resources.pb'
import type { WatchEventSpec } from '@/api/watch'
import { WatchFunc } from '@/api/watch'
import type { Metadata } from '@/api/v1alpha1/resource.pb'
import type { Runtime } from '@/api/common/omni.pb'

type Props<T> = {
  name: string
  type: string
  title: string
  resource: Metadata
  runtime: Runtime
  pointFn: (spec: T, old: T) => Record<string, number>
  animations?: boolean
  legend?: boolean
  dataLabels?: boolean
  stacked?: boolean
  stroke?: { curve: string; width: number | number[]; dashArray?: number | number[] }
  colors?: string[]
  tailEvents?: number
  context?: WatchContext
  totalFn?: (spec: T, old: T) => string
  minFn?: (spec: T, old: T) => number
  maxFn?: (spec: T, old: T) => number
  formatter?: (value: number) => string
}

const props = withDefaults(defineProps<Props<any>>(), {
  stroke: () => {
    return { curve: 'smooth', width: 2, dashArray: 0 }
  },
  colors: () => ['#FFB103', '#FF8B59'],
  tailEvents: () => 25,
})

const {
  name,
  resource,
  runtime,
  context,
  animations,
  legend,
  dataLabels,
  stroke,
  pointFn,
  colors,
  totalFn,
  tailEvents,
  minFn,
  maxFn,
  stacked,
  formatter,
} = toRefs(props)

const series: Ref<Record<string, any>[]> = ref([])
const seriesMap = {}
const points = {}
const flush = {}
const total = ref()

const min: Ref<number | undefined> = ref(undefined)
const max: Ref<number | undefined> = ref(undefined)

const handlePoint = (message: WatchResponse, spec: WatchEventSpec) => {
  if (message.event?.event_type != EventType.UPDATED) {
    return
  }

  const resource = spec.res
  const old = spec.old

  const data = pointFn.value(resource?.spec, old?.spec)

  if (totalFn?.value) {
    total.value = totalFn.value(resource?.spec, old?.spec)
  }

  if (minFn?.value) {
    min.value = minFn.value(resource?.spec, old?.spec)
  }

  if (maxFn?.value) {
    max.value = maxFn.value(resource?.spec, old?.spec)
  }

  for (const key in data) {
    if (!(key in seriesMap)) {
      series.value.push({
        name: key,
        data: [],
      })

      seriesMap[key] = {
        index: series.value.length - 1,
        version: 0,
      }
    }

    const version = resource?.metadata?.version || ''
    const meta = seriesMap[key]
    if (version <= meta.version) {
      continue
    }

    let point: number | number[] = data[key]
    const updated = resource?.metadata?.updated
    if (updated) {
      point = [DateTime.fromISO(updated.toString()).toMillis(), point]
    }

    clearTimeout(flush[meta.index])

    if (!points[meta.index]) points[meta.index] = []

    points[meta.index].push(point)
    meta.version = version

    flush[meta.index] = setTimeout(() => {
      let dst = series.value[meta.index].data

      dst = dst.concat(points[meta.index])

      if (dst.length >= tailEvents.value) {
        dst.splice(0, dst.length - tailEvents.value + 1)
      }

      series.value[meta.index].data = dst
      points[meta.index] = []
    }, 50)
  }
}

const w = new WatchFunc(handlePoint)

w.setup({
  resource: resource.value,
  runtime: runtime.value,
  tailEvents: tailEvents.value,
  context: context?.value,
})

const options = computed(() => {
  return {
    chart: {
      type: props.type,
      background: '#00000000',
      id: name.value,
      zoom: {
        enabled: false,
      },
      animations: {
        enabled: animations?.value,
      },
      toolbar: {
        show: false,
      },
      stacked: stacked?.value,
    },
    legend: {
      show: legend?.value,
      formatter: formatter?.value,
    },
    dataLabels: {
      enabled: dataLabels?.value,
    },
    stroke: stroke.value,
    tooltip: {
      theme: 'dark',
      x: {
        format: 'HH:mm:ss',
      },
      style: {
        fontSize: '12px',
        fontFamily: 'Roboto',
      },
    },
    colors: colors.value,
    fill: {
      type: 'gradient',
      gradient: {
        shadeIntensity: 1,
        opacityFrom: 0.4,
        opacityTo: 0.1,
        stops: [0, 90, 100],
      },
    },
    grid: {
      borderColor: '#272932',
      strokeDashArray: 10,
      xaxis: {
        lines: {
          show: true,
        },
      },
      yaxis: {
        lines: {
          show: true,
        },
      },
    },
    xaxis: {
      type: 'datetime',
      labels: {
        datetimeFormatter: {
          year: 'yyyy',
          month: "MMM 'yy",
          day: 'dd MMM',
          hour: 'HH:mm',
        },
        style: {
          colors: '#5B5C64',
          fontSize: '10px',
          fontFamily: 'Roboto',
          fontWeight: 500,
        },
      },
      axisBorder: {
        show: false,
      },
      axisTicks: {
        show: false,
      },
    },
    yaxis: {
      forceNiceScale: false,
      decimalsInFloat: 1,
      tickAmount: 4,
      min: min.value,
      max: max.value,
      labels: {
        formatter: formatter?.value,
        style: {
          colors: '#5B5C64',
          fontSize: '10px',
          fontFamily: 'Roboto',
          fontWeight: 500,
        },
      },
    },
  }
})

const err = w.err
const loading = w.loading
</script>

<style>
#chartContainer .apexcharts-tooltip {
  padding: 10px 12px;
  color: #ffff;
  background-color: #191b24;
}
#chartContainer .apexcharts-tooltip-title {
  color: #ffff;
  background-color: #191b24;
}
#chartContainer .apexcharts-xaxistooltip-bottom {
  display: none;
}
#chartContainer .apexcharts-tooltip .apexcharts-tooltip-series-group.active {
  background-color: #191b24;
}
</style>
