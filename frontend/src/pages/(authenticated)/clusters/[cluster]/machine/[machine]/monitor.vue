<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ArrowDownIcon } from '@heroicons/vue/24/solid'
import prettyBytes from 'pretty-bytes'
import { computed, ref, watchEffect } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { RuntimeContext } from '@/api/options'
import { withContext, withRuntime } from '@/api/options'
import {
  TalosCPUID,
  TalosCPUType,
  TalosMemoryID,
  TalosMemoryType,
  TalosPerfNamespace,
} from '@/api/resources'
import { MachineService, type ProcessInfo } from '@/api/talos/machine/machine.pb'
import type { CPUSpec, MemorySpec } from '@/api/talos/perf.pb'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import { useTitle } from '@/methods/title'
import NodesMonitorChart, {
  type ChartSample,
  type ChartSeries,
} from '@/views/Nodes/components/NodesMonitorChart.vue'

definePage({ name: 'NodeMonitor' })

const route = useRoute()

const processes = ref<Proc[]>([])
const context = computed<RuntimeContext>(() => ({
  cluster: route.params.cluster,
  node: route.params.machine,
}))

const headers = [
  { id: 'pid' },
  { id: 'state' },
  { id: 'threads' },
  { id: 'cpu', header: 'CPU %' },
  { id: 'mem', header: 'Memory %' },
  { id: 'virtualMemory', header: 'Virt Memory' },
  { id: 'residentMemory', header: 'Res Memory' },
  { id: 'cpuTime', header: 'Time+' },
  { id: 'command' },
] satisfies { id: keyof Proc; header?: string }[]

const sort = ref<keyof Proc>('cpu')
const sortReverse = ref(true)

const memTotal = ref(0)

const getCPUTotal = (stat: CPUSpec['cpuTotal']) => {
  return [
    stat?.idle,
    stat?.iowait,
    stat?.user,
    stat?.nice,
    stat?.system,
    stat?.irq,
    stat?.steal,
    stat?.softIrq,
  ].reduce<number>((prev, curr) => prev + (curr || 0), 0)
}

const prevProcs: Record<number, ProcessInfo> = {}
let prevCPU = 0

type Proc = NonNullable<Awaited<ReturnType<typeof getProcs>>>[number]
const getProcs = async () => {
  if (memTotal.value === 0) return

  const options = [withRuntime(Runtime.Talos), withContext(context.value)]

  const { messages: procMessages = [] } = await MachineService.Processes({}, ...options)
  const { messages: [systemStat] = [] } = await MachineService.SystemStat({}, ...options)

  const cpuTotal = getCPUTotal(systemStat.cpu_total ?? {}) / systemStat.cpu!.length

  const total = memTotal.value * 1024

  const procs = procMessages.flatMap(({ processes = [] }) =>
    processes.map((proc) => {
      let cpuDiff = 0

      if (prevProcs[proc.pid!]?.cpu_time && proc.cpu_time) {
        cpuDiff = proc.cpu_time - prevProcs[proc.pid!].cpu_time!
      }

      prevProcs[proc.pid!] = proc

      return {
        mem: (parseInt(proc.resident_memory || '0') / total) * 100,
        cpu: (cpuDiff / (cpuTotal - prevCPU)) * 100,
        threads: proc.threads!,
        pid: proc.pid!,
        state: proc.state!,
        virtualMemory: parseInt(proc.virtual_memory || '0'),
        residentMemory: parseInt(proc.resident_memory || '0'),
        command: processArgs(proc),
        cpuTime: proc.cpu_time || 0,
      }
    }),
  )

  prevCPU = cpuTotal

  return procs
}

// Based on talosctl processes cmd -> https://github.com/siderolabs/talos/blob/e9e027c6317aef55b8eb3463993ae6e856dcfc1c/cmd/talosctl/cmd/talos/processes.go#L105-L125
function processArgs(p: ProcessInfo) {
  const argsCmd = p.args?.trim().split(/\s+/).at(0)
  const execCmd = p.executable?.trim().split(/\s+/).at(0)

  let command

  if (!p.executable) {
    command = p.command
  } else if (argsCmd && argsCmd === execCmd?.split('/').pop()) {
    // Always use full command path if available
    command = p.args!.replace(argsCmd, execCmd)
  } else {
    command = p.args
  }

  // Replace control characters with spaces
  return command?.replace(/\p{Cc}/gu, ' ') ?? ''
}

async function loadProcs() {
  const procs = await getProcs()
  if (procs) processes.value = procs
}

watchEffect((onCleanup) => {
  loadProcs()

  const interval = window.setInterval(loadProcs, 5000)

  onCleanup(() => {
    clearInterval(interval)
  })
})

const cpuSeries: ChartSeries[] = [
  { key: 'system', label: 'System', color: 'var(--color-yellow-y1)' },
  { key: 'user', label: 'User', color: 'var(--color-primary-p3)' },
]

const memorySeries: ChartSeries[] = [
  { key: 'used', label: 'Used', color: 'var(--color-primary-p3)' },
  { key: 'cached', label: 'Cached', color: 'var(--color-naturals-n11)', width: 0.5, dash: 2 },
  { key: 'buffers', label: 'Buffers', color: 'var(--color-naturals-n11)', width: 0.5, dash: 2 },
]

const processSeries: ChartSeries[] = [
  { key: 'created', label: 'Created', color: 'var(--color-blue-b1)' },
  { key: 'running', label: 'Running', color: 'var(--color-green-g1)' },
  { key: 'blocked', label: 'Blocked', color: 'var(--color-yellow-y1)' },
]

const handleCPU = ({ spec, previous }: ChartSample<CPUSpec>) => {
  const keys = Object.keys(spec.cpuTotal ?? {}) as (keyof CPUSpec['cpuTotal'])[]

  const cpuTotal = keys.reduce<CPUSpec['cpuTotal']>(
    (prev, key) => ({
      ...prev,
      [key]: (spec.cpuTotal?.[key] || 0) - (previous.cpuTotal?.[key] || 0),
    }),
    {},
  )

  const total = getCPUTotal(cpuTotal)
  if (total === 0) {
    return { system: 0, user: 0 }
  }

  return {
    system: ((cpuTotal?.system || 0) / total) * 100,
    user: ((cpuTotal?.user || 0) / total) * 100,
  }
}

function formatPct(input: number) {
  // Re-parse the fixed string to drop a trailing .0
  const pct = Number(input.toFixed(1))

  return `${pct} %`
}

const handleTotalCPU = (sample: ChartSample<CPUSpec>) => {
  const { user, system } = handleCPU(sample)

  return formatPct(user + system)
}

const handleMem = ({ spec }: ChartSample<MemorySpec>) => {
  memTotal.value = spec.total || 0

  return {
    used: (spec.used || 0) - (spec.cached || 0) - (spec.buffers || 0),
    cached: spec.cached || 0,
    buffers: spec.buffers || 0,
  }
}

const handleTotalMem = ({ spec }: ChartSample<MemorySpec>) => {
  const used = (spec.used || 0) - (spec.cached || 0) - (spec.buffers || 0)

  return `${prettyBytes(used * 1024, { binary: true })} / ${prettyBytes((spec.total || 0) * 1024, { binary: true })}`
}

const handleMaxMem = ({ spec }: ChartSample<MemorySpec>) => {
  return spec.total || 0
}

const handleProcs = ({ spec, previous }: ChartSample<CPUSpec>) => {
  return {
    created: (spec.processCreated || 0) - (previous.processCreated || 0),
    running: spec.processRunning || 0,
    blocked: spec.processBlocked || 0,
  }
}

const sortedProcesses = computed(() => {
  return [...processes.value].sort((a, b) => {
    let res = 0
    if (a[sort.value] > b[sort.value]) {
      res = 1
    } else if (a[sort.value] < b[sort.value]) {
      res = -1
    }

    return sortReverse.value ? -1 * res : res
  })
})

const sortBy = (id: keyof Proc) => {
  if (id === sort.value) sortReverse.value = !sortReverse.value
  else sortReverse.value = true

  sort.value = id
}

useTitle('Monitor')
</script>

<template>
  <PageContainer class="monitor">
    <div class="monitor-charts-box">
      <div class="monitor-charts-wrapper">
        <div class="monitor-chart">
          <NodesMonitorChart
            class="h-full"
            title="CPU usage"
            :series="cpuSeries"
            :watch-opts="{
              runtime: Runtime.Talos,
              resource: {
                type: TalosCPUType,
                namespace: TalosPerfNamespace,
                id: TalosCPUID,
              },
              context,
            }"
            :point="handleCPU"
            :summary="handleTotalCPU"
            :min="0"
            :max="100"
            :format="formatPct"
            stacked
          />
        </div>
        <div class="monitor-chart">
          <NodesMonitorChart
            class="h-full"
            title="Memory"
            :series="memorySeries"
            :watch-opts="{
              runtime: Runtime.Talos,
              resource: {
                type: TalosMemoryType,
                namespace: TalosPerfNamespace,
                id: TalosMemoryID,
              },
              context,
            }"
            stacked
            :point="handleMem"
            :summary="handleTotalMem"
            :min="0"
            :max="handleMaxMem"
            :format="(value) => prettyBytes(value * 1024, { binary: true })"
          />
        </div>
      </div>
      <div class="monitor-charts-wrapper">
        <div class="monitor-chart monitor-chart-wide">
          <NodesMonitorChart
            class="h-full"
            title="Processes"
            :series="processSeries"
            :watch-opts="{
              runtime: Runtime.Talos,
              resource: {
                type: TalosCPUType,
                namespace: TalosPerfNamespace,
                id: TalosCPUID,
              },
              context,
            }"
            :point="handleProcs"
          />
        </div>
      </div>
    </div>
    <div class="monitor-data-wrapper">
      <div class="grid grid-cols-12 font-bold uppercase select-none">
        <div
          v-for="h in headers"
          :key="h.id"
          class="flex cursor-pointer flex-row items-center gap-1 text-center text-xs capitalize transition-colors hover:text-naturals-n10"
          @click="() => sortBy(h.id)"
        >
          <span>{{ h.header || h.id }}</span>
          <ArrowDownIcon
            v-if="sort === h.id"
            class="h-3 w-3"
            :class="{ transform: sortReverse, 'rotate-180': sortReverse }"
          />
        </div>
      </div>
      <div class="monitor-data-box">
        <div
          v-for="process in sortedProcesses"
          :key="process.pid"
          class="grid grid-cols-12 py-2 text-xs text-naturals-n12"
          :title="process.command"
        >
          <div>
            {{ process.pid }}
          </div>
          <div>
            {{ process.state }}
          </div>
          <div>
            {{ process.threads }}
          </div>
          <div>
            {{ process.cpu.toFixed(1) }}
          </div>
          <div>
            {{ process.mem.toFixed(1) }}
          </div>
          <div>
            {{ prettyBytes(process.virtualMemory, { binary: true }) }}
          </div>
          <div>
            {{ prettyBytes(process.residentMemory, { binary: true }) }}
          </div>
          <div>
            {{ process.cpuTime }}
          </div>
          <div class="col-span-4 truncate">{{ process.command }}</div>
        </div>
      </div>
    </div>
  </PageContainer>
</template>

<style scoped>
@reference "../../../../../../index.css";

.monitor {
  @apply flex flex-col justify-start pb-5;
}
.monitor-charts-box {
  @apply flex flex-col overflow-hidden;
  padding-bottom: 0 !important;
}
.monitor-charts-wrapper {
  @apply mb-6 flex flex-1 gap-2;
}
.monitor-charts-wrapper:last-of-type {
  @apply mb-0;
}
.monitor-chart {
  @apply flex-1 rounded bg-naturals-n2 p-3 pt-4;
}
.monitor-chart:nth-child(1) {
  @apply mr-3;
}
.monitor-chart:nth-child(2) {
  @apply ml-3;
}
.monitor-chart-wide {
  @apply border-b border-naturals-n5;
  margin-right: 0 !important;
  padding-bottom: 29px;
  border-radius: 4px 4px 0 0;
}
.monitor-data-wrapper {
  @apply flex w-full flex-1 flex-col overflow-hidden bg-naturals-n2 px-2 pt-5 text-xs text-naturals-n13 lg:px-8;
}
.monitor-data-box {
  @apply flex-1 overflow-x-auto bg-naturals-n2 py-3;
}
</style>
