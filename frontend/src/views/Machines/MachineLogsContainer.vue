<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRouteQuery } from '@vueuse/router'
import { computed, ref, watchEffect } from 'vue'

import type { Data } from '@/api/common/common.pb'
import { Runtime } from '@/api/common/omni.pb'
import type { Resource, Stream } from '@/api/grpc'
import { ManagementService } from '@/api/omni/management/management.pb'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { withContext, withRuntime } from '@/api/options'
import {
  DefaultNamespace,
  MachineStatusType,
  TalosContainersContainerdNamespace,
  TalosContainersLogPrefix,
} from '@/api/resources'
import type { ContainerStatusSpec } from '@/api/talos/containers.pb'
import { type LogsRequest, MachineService } from '@/api/talos/machine/machine.pb'
import LogViewer from '@/components/LogViewer/LogViewer.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import { stripAnsiColours } from '@/methods/ansi'
import type { LogLine } from '@/methods/logs'
import { DefaultLogParser, LineDelimitedLogParser, setupLogStream } from '@/methods/logs'
import { formatISO } from '@/methods/time'
import { supportsMaintenanceEvents, useMachineServices } from '@/methods/useMachineServices'
import { useResourceWatch } from '@/methods/useResourceWatch'

const {
  clusterId,
  machineId,
  service,
  containers = [],
} = defineProps<{
  clusterId?: string
  machineId: string
  service?: string
  /** The Talos containers the machine declares, offered beside its services. */
  containers?: Resource<ContainerStatusSpec>[]
}>()

const searchInput = useRouteQuery('q', '')
const logs = ref<LogLine[]>([])
const context = computed(() => ({
  cluster: clusterId,
  node: machineId,
}))

const MACHINE_LOGS = 'machine' // dummy value to represent non-service machine logs
const CONTROLLER_RUNTIME = 'controller-runtime' // service with logs that isn't reported in services list

// 'machine' check to continue support for /logs/machine but it is equivalent to /logs
const isMachineLogs = computed(() => !service || service === MACHINE_LOGS)

// A Talos container's log goes by the ID Talos registers it under, its name with a prefix, so it
// can't be mistaken for a service of the same name
const container = computed(() =>
  service?.startsWith(TalosContainersLogPrefix)
    ? service.slice(TalosContainersLogPrefix.length)
    : undefined,
)

const { data: machineStatus } = useResourceWatch<MachineStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    id: machineId,
    type: MachineStatusType,
    namespace: DefaultNamespace,
  },
}))

const supportsEvents = computed(
  () =>
    !!machineStatus.value?.spec.talos_version &&
    supportsMaintenanceEvents(
      machineStatus.value?.spec.maintenance ?? false,
      machineStatus.value.spec.talos_version,
    ),
)

const { data: services } = useMachineServices(context, supportsEvents)

const servicesSelectValues = computed(() => [
  MACHINE_LOGS,
  CONTROLLER_RUNTIME,
  ...services.value.map((s) => s.name ?? ''),
  ...containers.map(({ metadata: { id } }) => ({
    label: `${id} (container)`,
    value: `${TalosContainersLogPrefix}${id}`,
  })),
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
    const date = new Date(parseFloat(parsed.ts))

    return {
      date: date.toISOString(),
      msg: `[${parsed.level ?? 'info'}] ${parsed.msg} ${formatLoggingContext(parsed, 'msg', 'ts', 'level')}`,
    }
  },
  [CONTROLLER_RUNTIME](line) {
    // Format: <ISO timestamp> <ANSI>LEVEL<ANSI_RESET> <message> <JSON context>
    const stripped = stripAnsiColours(line)
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

// Talos registers a container's log when it first starts the task, and keeps it across restarts.
// Until then there is nothing to stream, so the page waits on the state instead.
const notStarted = computed(() => {
  const status = containers.find((c) => c.metadata.id === container.value)?.spec

  return (
    !!status &&
    !status.restartCount &&
    ['pending', 'pulling', 'starting'].includes(status.state ?? '')
  )
})

watchEffect((onCleanup) => {
  // Opens the stream the moment the container first starts, rather than waiting out a retry
  if (notStarted.value) {
    stream.value = undefined
    logs.value = []

    return
  }

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
            msg: data.msg.replace(/\n$/, ''),
          }
        }),
      )
    : setupLogStream(
        logs,
        MachineService.Logs,
        container.value
          ? {
              namespace: TalosContainersContainerdNamespace,
              id: container.value,
              follow: true,
              tail_lines: -1,
            }
          : {
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
    <div class="mb-compact flex gap-tight">
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
      v-if="notStarted"
      :title="`Waiting for ${container} to start`"
      type="info"
      class="mb-tight"
    >
      Its log appears here as soon as it starts.
    </TAlert>
    <TAlert
      v-else-if="stream?.err"
      :title="logs.length ? 'Disconnected' : 'Failed to Fetch Logs'"
      type="error"
      class="mb-tight"
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
