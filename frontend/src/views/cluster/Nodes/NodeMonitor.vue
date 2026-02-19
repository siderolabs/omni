<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ArrowDownIcon } from '@heroicons/vue/24/solid'
import { computed, onMounted, onUnmounted, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { withContext, withRuntime } from '@/api/options'
import {
  TalosCPUID,
  TalosCPUType,
  TalosMemoryID,
  TalosMemoryType,
  TalosPerfNamespace,
} from '@/api/resources'
import { MachineService, type ProcessInfo } from '@/api/talos/machine/machine.pb'
import { getContext } from '@/context'
import { formatBytes } from '@/methods'
import NodesMonitorChart from '@/views/cluster/Nodes/components/NodesMonitorChart.vue'

interface Diffable {
  [key: string]: number | Diffable | Diffable[]
}

function diff<T extends Diffable>(a: T, b: T): T {
  const result: Diffable = {}

  for (const key in a) {
    const left = a[key]
    const right = b[key]

    if (typeof left === 'number' && typeof right === 'number') {
      result[key] = left - right
    } else if (Array.isArray(left) && Array.isArray(right)) {
      result[key] = left.map((val, i) => diff(val, right[i]))
    } else if (
      !Array.isArray(left) &&
      !Array.isArray(right) &&
      typeof left === 'object' &&
      typeof right === 'object'
    ) {
      result[key] = diff(left, right)
    }
  }

  return result as T
}

const processes = ref<Proc[]>([])
const context = getContext()
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

let memTotal = 0
let interval: number

const sum = (obj: Record<string, number>, ...args: string[]) => {
  let res = 0
  for (const k of args) {
    res += obj[k] || 0
  }

  return res
}

const getCPUTotal = (stat: Record<string, number>) => {
  const idle = sum(stat, 'idle', 'iowait')
  const nonIdle = sum(stat, 'user', 'nice', 'system', 'irq', 'steal', 'softIrq')

  return idle + nonIdle
}

const prevProcs: Record<number, ProcessInfo> = {}
let prevCPU = 0

type Proc = NonNullable<Awaited<ReturnType<typeof getProcs>>>[number]
const getProcs = async () => {
  if (memTotal === 0) return

  const options = [withRuntime(Runtime.Talos), withContext(context)]

  const { messages: procMessages = [] } = await MachineService.Processes({}, ...options)
  const { messages: [systemStat] = [] } = await MachineService.SystemStat({}, ...options)

  const cpuTotal = getCPUTotal(systemStat.cpu_total ?? {}) / systemStat.cpu!.length

  const total = memTotal * 1024

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
        command: proc.command!,
        cpuTime: proc.cpu_time || 0,
        args: proc.args!,
      }
    }),
  )

  prevCPU = cpuTotal

  return procs
}

async function loadProcs() {
  const procs = await getProcs()
  if (procs) processes.value = procs
}

onMounted(() => {
  loadProcs()
  interval = window.setInterval(loadProcs, 5000)
})

onUnmounted(() => {
  clearInterval(interval)
})

const handleCPU = (oldObj: Diffable, newObj: Diffable) => {
  const delta = diff(oldObj, newObj)
  const stat = delta.cpuTotal as Record<string, number>
  const total = getCPUTotal(stat)

  return {
    system: (stat.system / total) * 100,
    user: (stat.user / total) * 100,
  }
}

const handleTotalCPU = (oldObj: Diffable, newObj: Diffable) => {
  const point = handleCPU(oldObj, newObj)

  return `${(point.user + point.system).toFixed(1)} %`
}

const handleMem = (
  _: unknown,
  m: { used: number; cached: number; buffers: number; total: number },
) => {
  const used = m.used - m.cached - m.buffers

  const memoryInitialized = memTotal === 0

  memTotal = m.total

  if (memoryInitialized) {
    loadProcs()
  }

  return {
    used: used,
    cached: m.cached,
    buffers: m.buffers,
  }
}

const handleTotalMem = (
  _: unknown,
  m: { used: number; cached: number; buffers: number; total: number },
) => {
  const used = m.used - m.cached - m.buffers

  return `${formatBytes(used * 1024)} / ${formatBytes(m.total * 1024)}`
}

const handleMaxMem = (_: unknown, m: { total: number }): number => {
  return m.total
}

const handleProcs = (oldObj: Diffable, newObj: Diffable) => {
  const { processCreated } = diff(oldObj, newObj)

  return {
    // The diff algorithm should never return only numbers Record<string, number> for this case
    // But due to how its typed, adding a fallback just incase
    created: typeof processCreated === 'number' ? processCreated : Number(processCreated),
    running: newObj.processRunning as number,
    blocked: newObj.processBlocked as number,
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
</script>

<template>
  <div class="monitor py-4">
    <div class="monitor-charts-box">
      <div class="monitor-charts-wrapper">
        <div class="monitor-chart">
          <NodesMonitorChart
            class="h-full"
            name="cpu"
            title="CPU usage"
            type="area"
            :runtime="Runtime.Talos"
            :resource="{
              type: TalosCPUType,
              namespace: TalosPerfNamespace,
              id: TalosCPUID,
            }"
            :context="context"
            :point-fn="handleCPU"
            :total-fn="handleTotalCPU"
            :min-fn="() => 0"
            :max-fn="() => 100"
          />
        </div>
        <div class="monitor-chart">
          <NodesMonitorChart
            class="h-full"
            name="mem"
            title="Memory"
            type="area"
            :stroke="{ curve: 'smooth', width: [2, 0.5, 0.5], dashArray: [0, 2, 2] }"
            :colors="[
              'var(--color-primary-p3)',
              'var(--color-naturals-n11)',
              'var(--color-naturals-n11)',
            ]"
            :runtime="Runtime.Talos"
            :resource="{
              type: TalosMemoryType,
              namespace: TalosPerfNamespace,
              id: TalosMemoryID,
            }"
            stacked
            :context="context"
            :point-fn="handleMem"
            :total-fn="handleTotalMem"
            :min-fn="() => 0"
            :max-fn="handleMaxMem"
            :formatter="
              (input) =>
                typeof input === 'number'
                  ? formatBytes(input * 1024)
                  : formatBytes(Number(input) * 1024)
            "
          />
        </div>
      </div>
      <div class="monitor-charts-wrapper">
        <div class="monitor-chart monitor-chart-wide">
          <NodesMonitorChart
            class="h-full"
            name="procs"
            title="Processes"
            type="area"
            :colors="['var(--color-blue-b1)', 'var(--color-green-g1)', 'var(--color-yellow-y1)']"
            :runtime="Runtime.Talos"
            :resource="{
              type: TalosCPUType,
              namespace: TalosPerfNamespace,
              id: TalosCPUID,
            }"
            :context="context"
            :point-fn="handleProcs"
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
          :title="process.command + ' ' + process.args"
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
            {{ formatBytes(process.virtualMemory) }}
          </div>
          <div>
            {{ formatBytes(process.residentMemory) }}
          </div>
          <div>
            {{ process.cpuTime }}
          </div>
          <div class="col-span-4 truncate">{{ process.command }} {{ process.args }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

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
  min-height: 220px;
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
