<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
export type AuditLogEvent =
  | 'k8s_access'
  | 'talos_access'
  | 'create'
  | 'destroy'
  | 'update_with_conflicts'
  | 'update'
  | 'teardown'

export interface AuditLogMsg {
  event_ts: number
  event_type: AuditLogEvent
  event_data: {
    [key: string]: unknown
    session: {
      [key: string]: string
      user_agent: string
    }
  }
  resource_type?: string
  resource_id?: string
}
</script>

<script setup lang="ts">
import { getLocalTimeZone, today } from '@internationalized/date'
import { useVirtualizer } from '@tanstack/vue-virtual'
import { differenceInDays } from 'date-fns'
import { type DateRange } from 'reka-ui'
import {
  type ComponentPublicInstance,
  computed,
  ref,
  shallowRef,
  type UnwrapRef,
  useTemplateRef,
  watch,
} from 'vue'

import AuditLogItem from '@/components/AuditLogs/AuditLogItem.vue'
import TButton from '@/components/Button/TButton.vue'
import DateRangePicker from '@/components/DateRangePicker/DateRangePicker.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import PageHeader from '@/components/PageHeader.vue'
import TAlert from '@/components/TAlert.vue'
import { useReadAuditLog } from '@/views/Settings/useReadAuditLog'

definePage({ name: 'AuditLogs' })

const dateRange = shallowRef<DateRange>({
  start: today(getLocalTimeZone()).subtract({ days: 3 }),
  end: today(getLocalTimeZone()).add({ days: 1 }),
})

const { data, loading } = useReadAuditLog(() => ({
  options: {
    start_time: dateRange.value.start?.toString(),
    end_time: dateRange.value.end?.toString(),
  },
}))

function downloadLogs() {
  const link = document.createElement('a')
  const url = window.URL.createObjectURL(new Blob(data.value, { type: 'application/json' }))
  link.href = url
  link.download = 'auditlog.jsonlog'
  link.click()
  window.URL.revokeObjectURL(url)
}

const expandedRows = ref(new Set<number>())

// Reset expanded items if filters change
watch(data, () => (expandedRows.value = new Set()))

function toggleRow(index: number) {
  if (expandedRows.value.has(index)) {
    expandedRows.value.delete(index)
  } else {
    expandedRows.value.add(index)
  }
}

const scrollContainer = useTemplateRef('scrollContainer')

type VirtualizerOptions = UnwrapRef<Parameters<typeof useVirtualizer>[0]>

const rowVirtualizer = useVirtualizer(
  computed<VirtualizerOptions>(() => ({
    count: data.value.length,
    getScrollElement: () => scrollContainer.value,
    estimateSize: () => 40,
    overscan: 5,
  })),
)

const virtualRows = computed(() => rowVirtualizer.value.getVirtualItems())
const totalSize = computed(() => rowVirtualizer.value.getTotalSize())

function measureElement(el: Element | ComponentPublicInstance | null) {
  if (el) rowVirtualizer.value.measureElement(el as Element)
}
</script>

<template>
  <PageContainer class="flex h-full flex-col">
    <div class="flex items-start justify-between gap-1">
      <PageHeader title="Audit logs" />
      <TButton variant="highlighted" :disabled="loading" @click="downloadLogs">
        Download audit logs
      </TButton>
    </div>

    <div class="mb-4 flex justify-end gap-2">
      <DateRangePicker v-model="dateRange" title="Date range" hidden-title />
    </div>

    <TAlert
      v-if="
        dateRange.start &&
        dateRange.end &&
        differenceInDays(
          dateRange.end.toDate(getLocalTimeZone()),
          dateRange.start.toDate(getLocalTimeZone()),
        ) >= 7
      "
      type="warn"
      title="Long date range"
      class="mb-4"
    >
      When downloading audit logs for longer time periods the file size may be excessively large
      which can negatively impact Omni performance. Consider shortening the date range.
    </TAlert>

    <div role="grid" class="flex min-h-0 flex-1 flex-col overflow-hidden text-xs text-naturals-n13">
      <div
        role="rowgroup"
        class="grid shrink-0 grid-cols-[36px_160px_180px_220px_1fr] gap-x-0.5 bg-naturals-n2 px-2 text-left"
      >
        <div role="columnheader" aria-hidden="true" class="py-2 uppercase"></div>
        <div role="columnheader" class="py-2 uppercase">Timestamp</div>
        <div role="columnheader" class="py-2 uppercase">Type</div>
        <div role="columnheader" class="py-2 uppercase">Resource</div>
        <div role="columnheader" class="py-2 uppercase">Actor</div>
      </div>

      <div ref="scrollContainer" role="rowgroup" class="min-h-0 flex-1 overflow-y-auto">
        <div class="relative" :style="{ height: `${totalSize}px` }">
          <div
            class="absolute top-0 left-0 w-full"
            :style="{ transform: `translateY(${virtualRows[0]?.start ?? 0}px)` }"
          >
            <div
              v-for="vRow in virtualRows"
              :key="vRow.key.toString()"
              :ref="measureElement"
              :data-index="vRow.index"
              class="grid grid-cols-[36px_160px_180px_220px_1fr] gap-x-0.5 border-t border-naturals-n5"
            >
              <AuditLogItem
                :data="data[vRow.index]"
                :model-value="expandedRows.has(vRow.index)"
                @update:model-value="toggleRow(vRow.index)"
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  </PageContainer>
</template>
