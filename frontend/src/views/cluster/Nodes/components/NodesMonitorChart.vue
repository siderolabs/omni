<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ExclamationCircleIcon } from '@heroicons/vue/24/outline'
import { DateTime } from 'luxon'
import type { Ref } from 'vue'
import { computed, ref, toRefs } from 'vue'
import ApexChart from 'vue3-apexcharts'

import type { Runtime } from '@/api/common/omni.pb'
import type { WatchResponse } from '@/api/omni/resources/resources.pb'
import { EventType } from '@/api/omni/resources/resources.pb'
import type { Metadata } from '@/api/v1alpha1/resource.pb'
import type { WatchContext, WatchEventSpec } from '@/api/watch'
import { WatchFunc } from '@/api/watch'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'

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
  if (message.event?.event_type !== EventType.UPDATED) {
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

<template>
  <div class="flex flex-col">
    <div class="flex justify-between">
      <div v-if="title" class="w-full pl-3 text-left text-xs text-naturals-n13">
        {{ title }}
      </div>
      <div v-if="total" class="w-full pr-3 text-right text-xs">
        {{ total }}
      </div>
    </div>
    <div id="chartContainer" class="flex-1">
      <div v-if="err || loading" class="flex h-full w-full flex-row items-center justify-center">
        <div
          v-if="err"
          class="flex w-1/2 items-center justify-center gap-4 text-sm text-neutral-500"
        >
          <div class="flex-0">
            <ExclamationCircleIcon class="h-6 w-6" />
          </div>
          <div>{{ err }}</div>
        </div>
        <TSpinner v-else class="h-5 w-5" />
      </div>
      <ApexChart
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
