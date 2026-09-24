<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
export interface ChartSeries {
  /** Matches a key returned by the chart's `point` function. */
  key: string
  /** Shown in the tooltip; defaults to the key. */
  label?: string
  color: string
  /** Stroke width of the line drawn on top of the area. */
  width?: number
  /** Dash length; omit for a solid line. */
  dash?: number
}

/** One watch update: the resource as it now is, and as it was before. */
export interface ChartSample<T> {
  spec: T
  previous: T
}
</script>

<script setup lang="ts" generic="T = unknown">
import { ExclamationCircleIcon } from '@heroicons/vue/24/outline'
import { useElementSize, useMouseInElement } from '@vueuse/core'
import { format as formatDate, parseISO } from 'date-fns'
import { computed, ref, shallowRef, useId, useTemplateRef, watch } from 'vue'

import { Code } from '@/api/google/rpc/code.pb'
import type { Resource } from '@/api/grpc'
import { EventType } from '@/api/omni/resources/resources.pb'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import { useResourceWatch, type WatchOptions } from '@/methods/useResourceWatch'
import * as chart from '@/views/Nodes/lib/nodesMonitorChartMath'

type Bound = chart.Bound<ChartSample<T>>

const { watchOpts, series, point, stacked, min, max, summary, format } = defineProps<{
  watchOpts: WatchOptions
  title: string
  /** Drawn from the outside in; the first series is the bottom of a stack. */
  series: ChartSeries[]
  /** A value per series key, derived from one update. */
  point: (sample: ChartSample<T>) => Record<string, number>
  stacked?: boolean
  min?: Bound
  /** Left open to grow with the data when absent. */
  max?: Bound
  /** Readout shown beside the title. */
  summary?: (sample: ChartSample<T>) => string
  format?: (value: number) => string
}>()

// --- Data collection -------------------------------------------------------

/** One row per update, holding every series' value for that moment. */
const samples = ref<{ t: number; values: number[] }[]>([])
const latest = shallowRef<ChartSample<T>>()
let lastVersion = 0

const tailEvents = computed(() => watchOpts.tailEvents ?? 25)

const handlePoint = (resource: Resource<T>, previous: Resource<T>) => {
  const version = Number(resource.metadata.version ?? '')
  // Guard against replayed events; the resource version only moves forwards.
  if (version <= lastVersion) return
  lastVersion = version

  const sample = { spec: resource.spec, previous: previous.spec }
  const values = point(sample)
  const updated = resource.metadata.updated

  latest.value = sample
  samples.value = chart.appendSample(
    samples.value,
    {
      t: updated ? parseISO(updated.toString()).getTime() : Date.now(),
      values: series.map((s) => values[s.key] ?? 0),
    },
    tailEvents.value,
  )
}

const { err, errCode, loading } = useResourceWatch<T>(
  () => ({
    ...watchOpts,
    tailEvents: tailEvents.value,
  }),
  {
    onMessage({ event }, { res, old }) {
      switch (event?.event_type) {
        case EventType.UPDATED:
          if (res && old) handlePoint(res, old)
          break
      }
    },
  },
)

watch(loading, (val) => {
  if (!val) return

  // Reset chart to initial state if watch restarts
  samples.value = []
  latest.value = undefined
  lastVersion = 0
})

const minValue = computed(() => chart.resolveBound(min, latest.value))
const maxValue = computed(() => chart.resolveBound(max, latest.value))
const readout = computed(() => (latest.value !== undefined ? summary?.(latest.value) : undefined))

// --- Layout ----------------------------------------------------------------

const plot = useTemplateRef('plot')
const { width, height } = useElementSize(plot)

const { MARGIN_TOP, MARGIN_RIGHT, MARGIN_BOTTOM } = chart

const formatValue = (value: number) => chart.formatValue(value, format)

const yDomain = computed(() =>
  chart.yDomain(samples.value, stacked, minValue.value, maxValue.value),
)

const yScale = computed(() =>
  chart.yScale(yDomain.value, height.value, maxValue.value === undefined),
)

const yTicks = computed(() =>
  chart.yTicks(
    chart.yTickValues(
      yScale.value,
      yDomain.value,
      minValue.value !== undefined && maxValue.value !== undefined,
    ),
    yScale.value,
    format,
  ),
)

const marginLeft = computed(() => chart.marginLeft(yTicks.value.map((tick) => tick.label)))

const xDomain = computed(() => chart.xDomain(samples.value))

const xScale = computed(() => chart.xScale(xDomain.value, width.value, marginLeft.value))

const xTicks = computed(() =>
  chart.xTicks(xScale.value, xDomain.value, width.value, marginLeft.value),
)

// --- Shapes ----------------------------------------------------------------

const bands = computed(() => chart.bands(samples.value, series.length, stacked))

const uid = useId()

const layers = computed(() => {
  const area = chart.areaPath(xScale.value, yScale.value)
  const line = chart.linePath(xScale.value, yScale.value)

  return series.map((s, index) => ({
    key: s.key,
    color: s.color,
    gradientId: `${uid}-${index}`,
    area: area(bands.value[index]) ?? '',
    line: line(bands.value[index]) ?? '',
    width: s.width ?? 2,
    dash: s.dash ? `${s.dash} ${s.dash}` : undefined,
  }))
})

// --- Tooltip ---------------------------------------------------------------

const { elementX, isOutside } = useMouseInElement(plot)

// Resolved from the cursor rather than stored on hover, so a point streaming in
// under a still cursor updates the readout instead of leaving it on a stale one.
const hovered = computed(() => {
  if (isOutside.value || !samples.value.length) return

  return chart.nearestIndex(samples.value, xScale.value, elementX.value)
})

const tooltip = computed(() => {
  const index = hovered.value
  if (index === undefined) return

  const sample = samples.value[index]
  if (!sample) return

  const rows = series.map((s, i) => ({
    key: s.key,
    label: s.label ?? s.key,
    color: s.color,
    value: formatValue(sample.values[i]),
  }))

  return {
    x: xScale.value(sample.t),
    time: formatDate(sample.t, 'HH:mm:ss'),
    // Stacked charts read top-down, matching the visual order of the bands.
    rows: stacked ? [...rows].reverse() : rows,
    markers: series.map((s, i) => ({
      key: s.key,
      color: s.color,
      y: yScale.value(bands.value[i][index].y1),
    })),
  }
})

const tip = useTemplateRef('tip')
const { width: tipWidth } = useElementSize(tip)

const tipLeft = computed(() =>
  chart.tooltipLeft(tooltip.value?.x ?? 0, tipWidth.value, width.value),
)
</script>

<template>
  <div class="flex flex-col">
    <div class="flex justify-between px-3 text-xs">
      <span v-if="title" class="text-naturals-n13">{{ title }}</span>
      <span v-if="readout">{{ readout }}</span>
    </div>

    <div class="relative h-45">
      <div v-if="err || loading" class="flex h-full items-center justify-center">
        <span v-if="err" class="flex items-center justify-center gap-4 text-sm text-naturals-n9">
          <ExclamationCircleIcon class="size-6" />
          {{ errCode === Code.UNAVAILABLE ? 'Talos API is not ready yet' : err }}
        </span>

        <TSpinner v-else class="size-5" />
      </div>

      <div v-show="!err && !loading" ref="plot" class="h-full w-full">
        <svg
          :width="width"
          :height="height"
          class="overflow-visible"
          role="img"
          :aria-label="title"
        >
          <defs>
            <linearGradient
              v-for="layer in layers"
              :id="layer.gradientId"
              :key="layer.gradientId"
              x1="0"
              y1="0"
              x2="0"
              y2="1"
            >
              <stop offset="0%" :style="{ stopColor: layer.color }" stop-opacity="0.4" />
              <stop offset="90%" :style="{ stopColor: layer.color }" stop-opacity="0.1" />
              <stop offset="100%" :style="{ stopColor: layer.color }" stop-opacity="0.1" />
            </linearGradient>
          </defs>

          <g class="stroke-naturals-n5" stroke-dasharray="10">
            <line
              v-for="tick in yTicks"
              :key="tick.value"
              :x1="marginLeft"
              :x2="width - MARGIN_RIGHT"
              :y1="tick.y"
              :y2="tick.y"
            />
            <line
              v-for="tick in xTicks"
              :key="tick.key"
              :x1="tick.x"
              :x2="tick.x"
              :y1="MARGIN_TOP"
              :y2="height - MARGIN_BOTTOM"
            />
          </g>

          <g class="fill-naturals-n8 text-[0.625rem] font-medium">
            <text
              v-for="tick in yTicks"
              :key="tick.value"
              :x="marginLeft - 8"
              :y="tick.y"
              text-anchor="end"
              dominant-baseline="central"
            >
              {{ tick.label }}
            </text>
            <text
              v-for="tick in xTicks"
              :key="tick.key"
              :x="tick.x"
              :y="height - 4"
              :text-anchor="tick.anchor"
            >
              {{ tick.label }}
            </text>
          </g>

          <g v-for="layer in layers" :key="layer.key">
            <path :d="layer.area" :fill="`url(#${layer.gradientId})`" />
            <path
              :d="layer.line"
              fill="none"
              stroke-linecap="round"
              stroke-linejoin="round"
              :stroke-width="layer.width"
              :stroke-dasharray="layer.dash"
              :style="{ stroke: layer.color }"
            />
          </g>

          <g v-if="tooltip">
            <line
              class="stroke-naturals-n8"
              :x1="tooltip.x"
              :x2="tooltip.x"
              :y1="MARGIN_TOP"
              :y2="height - MARGIN_BOTTOM"
            />
            <circle
              v-for="marker in tooltip.markers"
              :key="marker.key"
              :cx="tooltip.x"
              :cy="marker.y"
              r="3"
              class="stroke-naturals-n2"
              stroke-width="2"
              :style="{ fill: marker.color }"
            />
          </g>
        </svg>

        <div
          v-if="tooltip"
          ref="tip"
          class="pointer-events-none absolute top-2 flex flex-col gap-1 rounded bg-naturals-n3 px-3 py-2.5 text-xs whitespace-nowrap text-naturals-n14 shadow"
          :style="{ left: `${tipLeft}px` }"
        >
          <div class="text-naturals-n11">{{ tooltip.time }}</div>
          <div v-for="row in tooltip.rows" :key="row.key" class="flex items-center gap-2">
            <span
              aria-hidden="true"
              class="size-2 rounded-xs"
              :style="{ backgroundColor: row.color }"
            />
            <span class="text-naturals-n11">{{ row.label }}</span>
            <span class="ml-auto">{{ row.value }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
