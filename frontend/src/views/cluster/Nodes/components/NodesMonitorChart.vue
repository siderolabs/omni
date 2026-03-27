<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts" generic="T = unknown">
import 'apexcharts/area'

import { ExclamationCircleIcon } from '@heroicons/vue/24/outline'
import type { ApexOptions } from 'apexcharts'
import { DateTime } from 'luxon'
import type { Ref } from 'vue'
import { computed, ref } from 'vue'
import VueApexCharts from 'vue3-apexcharts/core'

import { Code } from '@/api/google/rpc/code.pb'
import type { WatchResponse } from '@/api/omni/resources/resources.pb'
import { EventType } from '@/api/omni/resources/resources.pb'
import type { WatchEventSpec, WatchOptions } from '@/api/watch'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import { getNonce } from '@/methods'
import { useResourceWatch } from '@/methods/useResourceWatch'

type Props<T> = {
  watchOpts: WatchOptions
  name: string
  title: string
  pointFn: (spec: T, old: T) => Record<string, number>
  animations?: boolean
  legend?: boolean
  dataLabels?: boolean
  stacked?: boolean
  stroke?: ApexOptions['stroke']
  colors?: string[]
  totalFn?: (spec: T, old: T) => string
  minFn?: (spec: T, old: T) => number
  maxFn?: (spec: T, old: T) => number
  formatter?: (value: string | number) => string
}

const {
  watchOpts,
  name,
  animations,
  legend,
  dataLabels,
  stroke = { curve: 'smooth', width: 2, dashArray: 0 },
  pointFn,
  colors = ['var(--color-yellow-y1)', 'var(--color-primary-p3)'],
  totalFn,
  minFn,
  maxFn,
  stacked,
  formatter,
} = defineProps<Props<T>>()

type Point = number | number[]
const series = ref<{ name: string; data: Point[] }[]>([])
const seriesMap: Record<string, { index: number; version: number }> = {}
const points: Record<number, Point[]> = {}
const flush: Record<number, number> = {}
const total = ref<string>()

const min: Ref<number | undefined> = ref(undefined)
const max: Ref<number | undefined> = ref(undefined)

const tailEvents = computed(() => watchOpts.tailEvents ?? 25)

const handlePoint = (message: WatchResponse, spec: WatchEventSpec) => {
  if (message.event?.event_type !== EventType.UPDATED) {
    return
  }

  const resource = spec.res
  const old = spec.old

  const data = pointFn(resource?.spec, old?.spec)

  if (totalFn) {
    total.value = totalFn(resource?.spec, old?.spec)
  }

  if (minFn) {
    min.value = minFn(resource?.spec, old?.spec)
  }

  if (maxFn) {
    max.value = maxFn(resource?.spec, old?.spec)
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

    const version = Number(resource?.metadata?.version ?? '')
    const meta = seriesMap[key]
    if (version <= meta.version) {
      continue
    }

    let point: Point = data[key]
    const updated = resource?.metadata?.updated
    if (updated) {
      point = [DateTime.fromISO(updated.toString()).toMillis(), point]
    }

    clearTimeout(flush[meta.index])

    if (!points[meta.index]) points[meta.index] = []

    points[meta.index].push(point)
    meta.version = version

    flush[meta.index] = window.setTimeout(() => {
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

const { err, errCode, loading } = useResourceWatch(
  () => ({
    ...watchOpts,
    tailEvents: tailEvents.value,
  }),
  handlePoint,
)

const options = computed(() => {
  return {
    chart: {
      nonce: getNonce(),
      background: 'transparent',
      id: name,
      zoom: {
        enabled: false,
      },
      animations: {
        enabled: animations,
      },
      toolbar: {
        show: false,
      },
      stacked: stacked,
    },
    legend: {
      show: legend,
      formatter: formatter,
    },
    dataLabels: {
      enabled: dataLabels,
    },
    stroke: stroke,
    tooltip: {
      theme: 'dark',
      x: {
        format: 'HH:mm:ss',
      },
      style: {
        fontSize: 'var(--text-xs)',
        fontFamily: 'var(--font-sans)',
      },
    },
    colors: colors,
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
      borderColor: 'var(--color-naturals-n5)',
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
          colors: 'var(--color-naturals-n8)',
          fontSize: '0.625rem',
          fontFamily: 'var(--font-sans)',
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
        formatter: formatter,
        style: {
          colors: 'var(--color-naturals-n8)',
          fontSize: '0.625rem',
          fontFamily: 'var(--font-sans)',
          fontWeight: 500,
        },
      },
    },
  } satisfies ApexOptions
})
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
          <div v-if="errCode === Code.UNAVAILABLE">Talos API is not ready yet</div>
          <div v-else>{{ err }}</div>
        </div>
        <TSpinner v-else class="h-5 w-5" />
      </div>
      <VueApexCharts
        v-show="!err && !loading"
        width="100%"
        height="100%"
        type="area"
        :options="options"
        :series="series ?? []"
      />
    </div>
  </div>
</template>

<style scoped>
@reference "../../../../index.css";

#chartContainer .apexcharts-tooltip {
  @apply bg-naturals-n3 px-3 py-2.5 text-naturals-n14;
}
#chartContainer .apexcharts-tooltip-title {
  @apply bg-naturals-n3 text-naturals-n14;
}
#chartContainer .apexcharts-xaxistooltip-bottom {
  @apply hidden;
}
#chartContainer .apexcharts-tooltip .apexcharts-tooltip-series-group.active {
  @apply bg-naturals-n3;
}
</style>
