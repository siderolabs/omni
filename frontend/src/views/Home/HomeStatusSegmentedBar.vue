<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useId } from 'vue'

interface Segment {
  label: string
  value: number
  color: string
}

interface Bar {
  label?: string
  segments: Segment[]
}

const { total, bars } = defineProps<{
  title: string
  total?: number
  bars: Bar[]
}>()

const labelId = useId()

function barTotal(bar: Bar) {
  return bar.segments.reduce((prev, curr) => prev + curr.value, 0)
}
</script>

<template>
  <div class="flex flex-col gap-snug">
    <div class="flex items-baseline justify-between gap-tight">
      <h2 :id="labelId" class="text-xl font-medium text-content-emphasis">{{ title }}</h2>
      <span v-if="total !== undefined" class="text-sm text-content-secondary">
        {{ total }} total
      </span>
    </div>

    <template v-for="(bar, barIndex) in bars" :key="bar.label ?? barIndex">
      <div class="flex flex-col gap-snug" :aria-labelledby="labelId">
        <div class="flex flex-col gap-micro">
          <span v-if="bar.label" class="text-xs text-content-secondary">{{ bar.label }}</span>

          <div
            class="flex h-2.5 w-full gap-0.5 overflow-hidden rounded-sm bg-surface-inert ring-1 ring-border-control ring-inset"
            role="img"
            :aria-label="
              bar.segments.map((segment) => `${segment.label}: ${segment.value}`).join(', ')
            "
          >
            <span
              v-for="segment in bar.segments.filter((s) => s.value > 0)"
              :key="segment.label"
              class="h-full transition-all"
              :style="{
                width: `${barTotal(bar) === 0 ? 0 : (segment.value / barTotal(bar)) * 100}%`,
                backgroundColor: segment.color,
              }"
            />
          </div>
        </div>
      </div>

      <dl class="flex flex-wrap gap-x-compact gap-y-1.5">
        <div
          v-for="(item, index) in bar.segments"
          :key="item.label"
          class="flex items-center gap-tight text-xs whitespace-nowrap"
        >
          <span
            aria-hidden="true"
            class="size-2 rounded-xs"
            :style="{ backgroundColor: item.color }"
          />
          <dt :id="`${labelId}-${barIndex}-dt-${index}`" class="text-content-secondary">
            {{ item.label }}
          </dt>
          <dd :aria-labelledby="`${labelId}-${barIndex}-dt-${index}`" class="text-content-emphasis">
            {{ item.value }}
          </dd>
        </div>
      </dl>
    </template>
  </div>
</template>
