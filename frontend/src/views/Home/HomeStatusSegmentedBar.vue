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
  <div class="flex flex-col gap-3">
    <div class="flex items-baseline justify-between gap-2">
      <h2 :id="labelId" class="text-xl font-medium text-naturals-n14">{{ title }}</h2>
      <span v-if="total !== undefined" class="text-sm text-naturals-n11">{{ total }} total</span>
    </div>

    <template v-for="(bar, barIndex) in bars" :key="bar.label ?? barIndex">
      <div class="flex flex-col gap-3" :aria-labelledby="labelId">
        <div class="flex flex-col gap-1">
          <span v-if="bar.label" class="text-xs text-naturals-n11">{{ bar.label }}</span>

          <div
            class="flex h-2.5 w-full gap-0.5 overflow-hidden rounded-sm bg-naturals-n5"
            role="img"
            :aria-label="
              bar.segments.map((segment) => `${segment.label}: ${segment.value}`).join(', ')
            "
          >
            <span
              v-for="segment in bar.segments"
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

      <dl class="flex flex-wrap gap-x-4 gap-y-1.5">
        <div
          v-for="(item, index) in bar.segments"
          :key="item.label"
          class="flex items-center gap-2 text-xs whitespace-nowrap"
        >
          <span
            aria-hidden="true"
            class="size-2 rounded-xs"
            :style="{ backgroundColor: item.color }"
          />
          <dt :id="`${labelId}-${barIndex}-dt-${index}`" class="text-naturals-n11">
            {{ item.label }}
          </dt>
          <dd :aria-labelledby="`${labelId}-${barIndex}-dt-${index}`" class="text-naturals-n14">
            {{ item.value }}
          </dd>
        </div>
      </dl>
    </template>
  </div>
</template>
