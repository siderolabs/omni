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
import { refDebounced } from '@vueuse/core'
import { useRouteQuery } from '@vueuse/router'
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

import { AuditLogOrderByDir, AuditLogOrderByField } from '@/api/omni/management/management.pb'
import AuditLogItem from '@/components/AuditLogs/AuditLogItem.vue'
import TButton from '@/components/Button/TButton.vue'
import DateRangePicker from '@/components/DateRangePicker/DateRangePicker.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import PageHeader from '@/components/PageHeader.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import { useReadAuditLog } from '@/views/Settings/useReadAuditLog'

definePage({ name: 'AuditLogs' })

const dateRange = shallowRef<DateRange>({
  start: today(getLocalTimeZone()).subtract({ days: 3 }),
  end: today(getLocalTimeZone()).add({ days: 1 }),
})

const searchInput = useRouteQuery('search', '')
const search = refDebounced(searchInput, 500)

const orderByField = useRouteQuery(
  'orderByField',
  AuditLogOrderByField.AUDIT_LOG_ORDER_BY_FIELD_DATE,
  { transform: (v) => Number(v) as AuditLogOrderByField },
)

const orderByDir = useRouteQuery('orderByDir', AuditLogOrderByDir.AUDIT_LOG_ORDER_BY_DIR_ASC, {
  transform: (v) => Number(v) as AuditLogOrderByDir,
})

const tableHeaders = [
  { label: 'Timestamp', value: AuditLogOrderByField.AUDIT_LOG_ORDER_BY_FIELD_DATE },
  { label: 'Type', value: AuditLogOrderByField.AUDIT_LOG_ORDER_BY_FIELD_EVENT_TYPE },
  { label: 'Resource', value: AuditLogOrderByField.AUDIT_LOG_ORDER_BY_FIELD_RESOURCE_TYPE },
  { label: 'Actor', value: AuditLogOrderByField.AUDIT_LOG_ORDER_BY_FIELD_ACTOR },
]

function toggleSort(value: AuditLogOrderByField) {
  if (orderByField.value !== value) {
    orderByField.value = value
    orderByDir.value = AuditLogOrderByDir.AUDIT_LOG_ORDER_BY_DIR_ASC
  } else if (orderByDir.value === AuditLogOrderByDir.AUDIT_LOG_ORDER_BY_DIR_ASC) {
    orderByDir.value = AuditLogOrderByDir.AUDIT_LOG_ORDER_BY_DIR_DESC
  } else {
    orderByDir.value = AuditLogOrderByDir.AUDIT_LOG_ORDER_BY_DIR_ASC
  }
}

const { data, loading, error } = useReadAuditLog(() => ({
  options: {
    start_time: dateRange.value.start?.toString(),
    end_time: dateRange.value.end?.toString(),
    search: search.value,
    order_by_field: orderByField.value,
    order_by_dir: orderByDir.value,
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

    <TAlert v-if="error" type="error" title="Error" class="mb-4">{{ error.message }}</TAlert>

    <TInput v-model="searchInput" class="mb-2" title="Search" />

    <div class="mb-4 flex justify-end gap-2">
      <DateRangePicker v-model="dateRange" title="Date range" inline-title />
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
        <div
          v-for="{ label, value } in tableHeaders"
          :key="value"
          role="columnheader"
          class="cursor-pointer py-2 uppercase transition-colors select-none hover:text-naturals-n11 active:text-naturals-n9"
          @click="toggleSort(value)"
        >
          <span class="inline-flex items-center">
            {{ label }}

            <TIcon
              class="size-5 text-naturals-n10 transition-transform"
              icon="dropdown"
              :class="{
                invisible: orderByField !== value,
                'rotate-180': orderByDir === AuditLogOrderByDir.AUDIT_LOG_ORDER_BY_DIR_ASC,
              }"
            />
          </span>
        </div>
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
                :search
                @update:model-value="toggleRow(vRow.index)"
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  </PageContainer>
</template>
