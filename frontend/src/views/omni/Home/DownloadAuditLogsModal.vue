<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { getLocalTimeZone, today } from '@internationalized/date'
import { differenceInDays } from 'date-fns'
import { type DateRange, type DateValue } from 'reka-ui'
import { onUnmounted, ref, shallowRef, watchEffect } from 'vue'

import { b64Decode } from '@/api/fetch.pb'
import { ManagementService } from '@/api/omni/management/management.pb'
import { withAbortController } from '@/api/options'
import DateRangePicker from '@/components/common/DateRangePicker/DateRangePicker.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { formatBytes } from '@/methods'
import { showError, showWarning } from '@/notification'
import Modal from '@/views/omni/Modals/Modal.vue'

const open = defineModel<boolean>('open', { default: false })

const downloadedBytes = ref(0)
const downloading = ref(false)
const done = ref(false)

const dateRange = shallowRef<DateRange>({
  start: today(getLocalTimeZone()).subtract({ days: 3 }) as unknown as DateValue,
  end: today(getLocalTimeZone()).add({ days: 1 }) as unknown as DateValue,
})

let abortController: AbortController

onUnmounted(() => {
  abortController?.abort()
})

watchEffect(() => {
  if (!open.value) {
    abortController?.abort()
    return
  }

  // Reset the modal when opening
  dateRange.value = {
    start: today(getLocalTimeZone()).subtract({ days: 3 }) as unknown as DateValue,
    end: today(getLocalTimeZone()).add({ days: 1 }) as unknown as DateValue,
  }

  downloadedBytes.value = 0
  downloading.value = false
  done.value = false
})

async function download() {
  try {
    downloadedBytes.value = 0
    downloading.value = true
    done.value = false

    abortController?.abort()
    abortController = new AbortController()

    const result: Uint8Array<ArrayBuffer>[] = []

    await ManagementService.ReadAuditLog(
      {
        start_time: dateRange.value.start!.toString(),
        end_time: dateRange.value.end!.toString(),
      },
      (resp) => {
        const data = resp.audit_log as unknown as string // audit_log is actually not a Uint8Array, but a base64 string
        const chunk = b64Decode(data) as Uint8Array<ArrayBuffer>

        downloadedBytes.value += chunk.byteLength

        result.push(chunk)
      },
      withAbortController(abortController),
    )

    if (!downloadedBytes.value) {
      showWarning('Warning', 'No logs found for the specified range')
      return
    }

    const link = document.createElement('a')
    link.href = window.URL.createObjectURL(new Blob(result, { type: 'application/json' }))
    link.download = 'auditlog.jsonlog'
    link.click()

    done.value = true
  } catch (e) {
    if (abortController.signal.aborted) return

    showError('Failed to download audit log', e instanceof Error ? e.message : String(e))
  } finally {
    downloading.value = false
  }
}
</script>

<template>
  <Modal
    v-model:open="open"
    title="Download Audit Logs"
    :cancel-label="done ? 'Close' : 'Cancel'"
    action-label="Download"
    :action-disabled="!dateRange.start || !dateRange.end"
    :loading="downloading"
    @confirm="download"
  >
    <template #description>Download audit logs for a specific date range</template>

    <div class="space-y-4">
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
      >
        When downloading audit logs for longer time periods the file size may be excessively large
        which can negatively impact Omni performance. Consider shortening the date range.
      </TAlert>

      <DateRangePicker v-model="dateRange" title="Date range" />

      <div v-if="downloading || done" class="flex items-center gap-2 text-sm">
        <TSpinner v-if="downloading" class="size-4 overflow-hidden" />
        <TIcon v-if="done" icon="check-in-circle" class="size-4 text-green-g2" />
        <p>
          <span>{{ downloading ? 'Downloading...' : 'Downloaded' }}</span>
          {{ formatBytes(downloadedBytes) }}
        </p>
      </div>
    </div>
  </Modal>
</template>
