<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { DateTime } from 'luxon'
import { computed, ref, watchEffect } from 'vue'

import type { Data } from '@/api/common/common.pb'
import { Runtime } from '@/api/common/omni.pb'
import type { Stream } from '@/api/grpc'
import { ManagementService } from '@/api/omni/management/management.pb'
import { withContext, withRuntime } from '@/api/options'
import { type LogsRequest, MachineService } from '@/api/talos/machine/machine.pb'
import LogViewer from '@/components/LogViewer/LogViewer.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import type { LogLine } from '@/methods/logs'
import { DefaultLogParser, LineDelimitedLogParser, setupLogStream } from '@/methods/logs'
import { formatISO } from '@/methods/time'
import { useMachineServices } from '@/methods/useMachineServices'

const { clusterId, machineId, service } = defineProps<{
  clusterId?: string
  machineId: string
  service?: string
}>()

const searchInput = ref('')
const logs = ref<LogLine[]>([])
const context = computed(() => ({
  cluster: clusterId,
  node: machineId,
}))

const MACHINE_LOGS = 'machine' // dummy value to represent non-service machine logs
const CONTROLLER_RUNTIME = 'controller-runtime' // service with logs that isn't reported in services list

// 'machine' check to continue support for /logs/machine but it is equivalent to /logs
const isMachineLogs = computed(() => !service || service === MACHINE_LOGS)

const { services } = useMachineServices(context)

const servicesSelectValues = computed(() => [
  MACHINE_LOGS,
  CONTROLLER_RUNTIME,
  ...services.value.map((s) => s.name ?? ''),
])

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
  containerd(line) {
    const parsed = JSON.parse(line)

    return {
      date: parsed.time,
      msg: `[${parsed.level ?? 'info'}] ${parsed.msg} ${formatLoggingContext(parsed, 'msg', 'ts', 'level')}`,
    }
  },
  cri(line) {
    const parsed = JSON.parse(line)

    return {
      date: parsed.time,
      msg: `[${parsed.level ?? 'info'}] ${parsed.msg} ${formatLoggingContext(parsed, 'msg', 'ts', 'level')}`,
    }
  },
  etcd(line) {
    const parsed = JSON.parse(line)

    return {
      date: parsed.ts,
      msg: `[${parsed.level ?? 'info'}] ${parsed.msg} ${formatLoggingContext(parsed, 'msg', 'ts', 'level')}`,
    }
  },
  kubelet(line) {
    const parsed = JSON.parse(line)
    const date = DateTime.fromSeconds(parseFloat(parsed.ts) / 1000)

    return {
      date: date.toISO() ?? undefined,
      msg: `[${parsed.level ?? 'info'}] ${parsed.msg} ${formatLoggingContext(parsed, 'msg', 'ts', 'level')}`,
    }
  },
  [CONTROLLER_RUNTIME](line) {
    // Format: <ISO timestamp> <ANSI>LEVEL<ANSI_RESET> <message> <JSON context>
    const stripped = line.replace(/\x1b\[[\d;]*m/g, '')
    const match = stripped.match(/^(\S+)\s+(\w+)\s+(.*?)\s+(\{.*\})$/)
    if (!match) throw new Error('Unexpected line format')

    const [, date, level, msg, contextJson] = match
    const parsed = JSON.parse(contextJson)

    return {
      date,
      msg: `[${level.toLowerCase()}] ${msg} ${formatLoggingContext(parsed, 'component')}`,
    }
  },
}

const plainText = (line: string) => {
  return {
    msg: line,
  }
}

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

const stream = ref<Stream<Data, LogsRequest>>()

watchEffect((onCleanup) => {
  const { stream: newStream, shutdown } = isMachineLogs.value
    ? setupLogStream(
        logs,
        ManagementService.MachineLogs,
        {
          machine_id: context.value.node,
          follow: true,
          tail_lines: -1,
        },
        new DefaultLogParser((line) => {
          const data = JSON.parse(line)

          return {
            date: formatISO(data['talos-time'], 'dd/MM/yyyy HH:mm:ss'),
            msg: data.msg,
          }
        }),
      )
    : setupLogStream(
        logs,
        MachineService.Logs,
        {
          namespace: 'system',
          id: service,
          follow: true,
          tail_lines: -1,
        },
        new LineDelimitedLogParser(getLineParser(service!)),
        withRuntime(Runtime.Talos),
        withContext(context.value),
      )

  stream.value = newStream

  onCleanup(() => shutdown())
})
</script>

<template>
  <div class="flex flex-col">
    <div class="mb-4 flex gap-2">
      <TSelectList
        title="Logs"
        :values="servicesSelectValues"
        :model-value="service || MACHINE_LOGS"
        @update:model-value="
          (v) => $router.replace({ params: { service: v === MACHINE_LOGS ? '' : v } })
        "
      />

      <TInput v-model="searchInput" placeholder="Search..." icon="search" class="grow" />
    </div>
    <TAlert
      v-if="stream?.err"
      :title="logs ? 'Disconnected' : 'Failed to Fetch Logs'"
      type="error"
      class="mb-2"
    >
      {{ stream.err }}
    </TAlert>
    <LogViewer
      class="flex-1"
      :logs="logs"
      :search-option="searchInput"
      :without-date="!isMachineLogs && !parsers[service!]"
    />
  </div>
</template>
