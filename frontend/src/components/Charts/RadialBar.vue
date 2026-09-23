<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, useId } from 'vue'

interface Props {
  title: string
  total?: number
  items: {
    label: string
    value: number
  }[]
  legendFormatter?: (value: number) => string
}

const {
  total: propsTotal,
  items,
  legendFormatter = (value) => value.toString(),
} = defineProps<Props>()

// Geometry in viewBox units, one per pixel. Rings run outside in, so the first
// item gets the outermost ring.
const VIEWBOX_WIDTH = 200
const VIEWBOX_HEIGHT = 170
const CENTER_X = VIEWBOX_WIDTH / 2
const CENTER_Y = VIEWBOX_HEIGHT / 2
const OUTER_RADIUS = 65
const RING_WIDTH = 8
const RING_GAP = 2
// Narrower than the bar, so the bar covers it wherever it is filled.
const TRACK_WIDTH = RING_WIDTH * 0.97

const colors = [
  'var(--color-primary-p3)',
  'var(--color-red-r1)',
  'var(--color-green-g1)',
  'var(--color-blue-b1)',
  'var(--color-yellow-y1)',
]

const trackColor = 'var(--color-naturals-n8)'

const total = computed(() => propsTotal ?? items.reduce((prev, curr) => prev + curr.value, 0))

const rings = computed(() =>
  items.map((item, index) => {
    const radius = OUTER_RADIUS - RING_WIDTH / 2 - index * (RING_WIDTH + RING_GAP)
    const circumference = 2 * Math.PI * radius
    const percent = total.value === 0 ? 0 : Math.round((item.value / total.value) * 100)

    return {
      label: item.label,
      radius,
      circumference,
      // Zero still shows a dot: the round line cap draws even on an empty arc.
      filled: (percent / 100) * circumference,
      color: colors[index],
    }
  }),
)

const legendItems = computed(() => [
  {
    label: 'Total',
    value: legendFormatter(total.value),
    color: trackColor,
  },
  ...items.map((item, i) => ({
    label: item.label,
    value: legendFormatter(item.value),
    color: colors[i],
  })),
])

const labelId = useId()
</script>

<template>
  <div class="flex flex-col gap-2">
    <h2 :id="labelId" class="text-xl font-medium text-naturals-n14">{{ title }}</h2>

    <figure
      class="flex flex-col items-center gap-2 self-center py-2 not-visited:px-4"
      :aria-labelledby="labelId"
    >
      <svg
        aria-hidden="true"
        :width="VIEWBOX_WIDTH"
        :height="VIEWBOX_HEIGHT"
        :viewBox="`0 0 ${VIEWBOX_WIDTH} ${VIEWBOX_HEIGHT}`"
        xmlns="http://www.w3.org/2000/svg"
      >
        <!-- Start arcs at 12 o'clock, running clockwise. -->
        <g :transform="`rotate(-90 ${CENTER_X} ${CENTER_Y})`">
          <circle
            v-if="rings.length"
            :cx="CENTER_X"
            :cy="CENTER_Y"
            :r="rings[0].radius"
            fill="none"
            :stroke-width="TRACK_WIDTH"
            :style="{ stroke: trackColor }"
          />

          <circle
            v-for="ring in rings"
            :key="ring.label"
            :cx="CENTER_X"
            :cy="CENTER_Y"
            :r="ring.radius"
            fill="none"
            stroke-linecap="round"
            :stroke-width="RING_WIDTH"
            :stroke-dasharray="`${ring.filled} ${ring.circumference}`"
            :style="{ stroke: ring.color }"
          />
        </g>
      </svg>

      <figcaption class="flex flex-col gap-2">
        <dl
          v-for="(item, index) in legendItems"
          :key="item.label"
          class="flex items-center gap-2 text-xs whitespace-nowrap"
        >
          <span
            aria-hidden="true"
            class="size-2 rounded-xs"
            :style="{ backgroundColor: item.color }"
          />
          <dt :id="`${labelId}-dt-${index}`" class="text-naturals-n11">{{ item.label }}</dt>
          <dd :aria-labelledby="`${labelId}-dt-${index}`" class="text-naturals-n14">
            {{ item.value }}
          </dd>
        </dl>
      </figcaption>
    </figure>
  </div>
</template>
