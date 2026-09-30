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
// The track's outline is narrower than the bar, so the bar covers it wherever
// it is filled. The track sits inside the outline, a hairline in from each edge.
const TRACK_OUTLINE_WIDTH = RING_WIDTH * 0.97
const TRACK_WIDTH = TRACK_OUTLINE_WIDTH - 2

const colors = [
  'var(--color-series-1)',
  'var(--color-series-2)',
  'var(--color-series-3)',
  'var(--color-series-4)',
  'var(--color-series-5)',
]

const trackColor = 'var(--color-surface-inert)'
const trackOutlineColor = 'var(--color-border-control)'

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
    outlined: true,
  },
  ...items.map((item, i) => ({
    label: item.label,
    value: legendFormatter(item.value),
    color: colors[i],
    outlined: false,
  })),
])

const labelId = useId()
</script>

<template>
  <div class="flex flex-col gap-2">
    <h2 :id="labelId" class="text-xl font-medium text-content-emphasis">{{ title }}</h2>

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
          <template v-for="ring in rings" :key="`track-${ring.label}`">
            <circle
              :cx="CENTER_X"
              :cy="CENTER_Y"
              :r="ring.radius"
              fill="none"
              :stroke-width="TRACK_OUTLINE_WIDTH"
              :style="{ stroke: trackOutlineColor }"
            />
            <circle
              :cx="CENTER_X"
              :cy="CENTER_Y"
              :r="ring.radius"
              fill="none"
              :stroke-width="TRACK_WIDTH"
              :style="{ stroke: trackColor }"
            />
          </template>

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
            :class="{ 'ring-1 ring-border-control ring-inset': item.outlined }"
            :style="{ backgroundColor: item.color }"
          />
          <dt :id="`${labelId}-dt-${index}`" class="text-content-secondary">{{ item.label }}</dt>
          <dd :aria-labelledby="`${labelId}-dt-${index}`" class="text-content-emphasis">
            {{ item.value }}
          </dd>
        </dl>
      </figcaption>
    </figure>
  </div>
</template>
