<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts" generic="T">
import { useElementSize } from '@vueuse/core'
import { computed, useTemplateRef } from 'vue'

import TableRoot from '@/components/Table/TableRoot.vue'
import TableRow from '@/components/Table/TableRow.vue'

const { minWidthRem } = defineProps<{
  rows: T[]
  rowKey: (row: T) => PropertyKey
  /**
   * The narrowest the table can be, in rem like the column widths it holds. Narrower than this,
   * the rows stack instead of the table scrolling sideways.
   */
  minWidthRem: number
}>()

defineSlots<{
  /** The header cells. */
  head(): unknown
  /** One row's cells in the table. */
  row(props: { row: T }): unknown
  /** One row stacked: what it says when there's no room for the columns. */
  stacked(props: { row: T }): unknown
}>()

// Measured rather than read from the viewport, so the sidebar and any other layout around the
// table count. Starts wide so a desktop doesn't flash the stacked rows first.
const container = useTemplateRef('container')
const { width } = useElementSize(container, { width: Number.POSITIVE_INFINITY, height: 0 })
const stacked = computed(() => {
  const remPx = parseFloat(getComputedStyle(document.documentElement).fontSize)

  return width.value < minWidthRem * remPx
})
</script>

<template>
  <div ref="container">
    <ul
      v-if="stacked"
      class="divide-y divide-border-default rounded border border-border-default bg-surface-card text-xs text-content-default"
    >
      <li v-for="row in rows" :key="rowKey(row)" class="px-compact py-tight">
        <slot name="stacked" :row />
      </li>
    </ul>

    <div v-else class="overflow-x-auto">
      <TableRoot class="w-full table-fixed" :style="{ minWidth: `${minWidthRem}rem` }">
        <template #head>
          <TableRow>
            <slot name="head" />
          </TableRow>
        </template>

        <template #body>
          <TableRow v-for="row in rows" :key="rowKey(row)">
            <slot name="row" :row />
          </TableRow>
        </template>
      </TableRoot>
    </div>
  </div>
</template>
