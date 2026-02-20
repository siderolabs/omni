<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { DateTime } from 'luxon'
import type { Ref } from 'vue'
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { withContext, withRuntime } from '@/api/options'
import type { LogsRequest } from '@/api/talos/machine/machine.pb'
import { MachineService } from '@/api/talos/machine/machine.pb'
import LogViewer from '@/components/common/LogViewer/LogViewer.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import TAlert from '@/components/TAlert.vue'
import { getContext } from '@/context'
import type { LogLine } from '@/methods/logs'
import { LineDelimitedLogParser, setupLogStream } from '@/methods/logs'
import MachineLogsContainer from '@/views/omni/Machines/MachineLogsContainer.vue'

const route = useRoute()
const inputValue = ref('')
const logs: Ref<LogLine[]> = ref([])
const context = getContext()
const service = ref(route.params.service as string)

const formatLoggingContext = (logRecord: Record<string, string>, ...exceptFields: string[]) => {
  const res: string[] = []

  for (const key in logRecord) {
    if (exceptFields.includes(key)) {
      continue
    }

    res.push(`${key}=${logRecord[key]}`)
  }

  return res.join(' ')
}

const parsers: Record<string, (line: string) => LogLine> = {
  containerd: (line: string): LogLine => {
    const parsed = JSON.parse(line)

    return {
      date: parsed.time,
      msg: `[${parsed.level ?? 'info'}] ${parsed.msg} ${formatLoggingContext(parsed, 'msg', 'ts', 'level')}`,
    }
  },
  cri: (line: string): LogLine => {
    const parsed = JSON.parse(line)

    return {
      date: parsed.time,
      msg: `[${parsed.level ?? 'info'}] ${parsed.msg} ${formatLoggingContext(parsed, 'msg', 'ts', 'level')}`,
    }
  },
  etcd: (line: string): LogLine => {
    const parsed = JSON.parse(line)

    return {
      date: parsed.ts,
      msg: `[${parsed.level ?? 'info'}] ${parsed.msg} ${formatLoggingContext(parsed, 'msg', 'ts', 'level')}`,
    }
  },
  kubelet: (line: string): LogLine => {
    const parsed = JSON.parse(line)
    const date = DateTime.fromSeconds(parseFloat(parsed.ts) / 1000)

    return {
      date: date.toISO() ?? undefined,
      msg: `[${parsed.level ?? 'info'}] ${parsed.msg} ${formatLoggingContext(parsed, 'msg', 'ts', 'level')}`,
    }
  },
}

const plainText = (line: string) => {
  return {
    msg: line,
  }
}

const params = computed<LogsRequest | undefined>(() => {
  if (route.params.service === 'machine') {
    return
  }

  return {
    namespace: 'system',
    id: service.value,
    follow: true,
    tail_lines: -1,
  }
})

const getLineParser = (svc: string) => {
  return parsers[svc]
    ? (line: string): LogLine => {
        try {
          return parsers[svc](line)
        } catch {
          return plainText(line)
        }
      }
    : plainText
}

const logParser = new LineDelimitedLogParser(getLineParser(service.value))

watch(
  () => route.params.service,
  () => {
    const svc = route.params.service as string

    logParser.setLineParser(getLineParser(svc))
    service.value = svc
  },
)

const stream = setupLogStream(
  logs,
  MachineService.Logs,
  params,
  logParser,
  withRuntime(Runtime.Talos),
  withContext(context),
)

const err = computed(() => {
  return stream.value?.err
})
</script>

<template>
  <div class="logs py-4">
    <MachineLogsContainer
      v-if="$route.params.service === 'machine'"
      :machine-id="route.params.machine as string"
      class="logs-container"
    />
    <div v-else class="logs-container">
      <div class="mb-4">
        <TInput v-model="inputValue" placeholder="Search..." icon="search" />
      </div>
      <TAlert
        v-if="err"
        :title="logs ? 'Disconnected' : 'Failed to Fetch Logs'"
        type="error"
        class="mb-2"
      >
        {{ err }}
      </TAlert>
      <LogViewer
        :logs="logs"
        :search-option="inputValue"
        class="flex-1"
        :without-date="!parsers[$route.params.service as string]"
      />
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.logs {
  @apply flex h-full flex-col;
  max-height: calc(100vh - 150px);
  overflow: hidden;
}
.logs-container {
  @apply flex grow flex-col;
}
</style>
