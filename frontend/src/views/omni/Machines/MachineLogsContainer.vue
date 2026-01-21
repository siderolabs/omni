<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { ref } from 'vue'
import { useRoute } from 'vue-router'

import { ManagementService } from '@/api/omni/management/management.pb'
import LogViewer from '@/components/common/LogViewer/LogViewer.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import type { LogLine } from '@/methods/logs'
import { DefaultLogParser, setupLogStream } from '@/methods/logs'
import { formatISO } from '@/methods/time'

const logs: Ref<LogLine[]> = ref([])
const route = useRoute()

const searchInput = ref('')

interface Props {
  machineId?: string
}

const props = defineProps<Props>()

setupLogStream(
  logs,
  ManagementService.MachineLogs,
  {
    machine_id: props.machineId || (route.params.machine as string),
    tail_lines: -1,
    follow: true,
  },
  new DefaultLogParser((line: string): LogLine => {
    const data = JSON.parse(line)

    return {
      date: formatISO(data['talos-time'], 'dd/MM/yyyy HH:mm:ss'),
      msg: data.msg,
    }
  }),
)
</script>

<template>
  <div class="flex flex-col">
    <div class="mb-4">
      <TInput v-model="searchInput" placeholder="Search..." icon="search" />
    </div>
    <LogViewer class="flex-1" :logs="logs" :search-option="searchInput" />
  </div>
</template>
