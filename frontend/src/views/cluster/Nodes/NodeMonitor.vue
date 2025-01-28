<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="monitor">
    <div class="monitor-charts-box">
      <div class="monitor-charts-wrapper">
        <div class="monitor-chart">
          <nodes-monitor-chart
            class="h-full"
            name="cpu"
            title="CPU usage"
            type="area"
            :colors="['#FFB103', '#FF8B59']"
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
          <nodes-monitor-chart
            class="h-full"
            name="mem"
            title="Memory"
            type="area"
            :stroke="{ curve: 'smooth', width: [2, 0.5, 0.5], dashArray: [0, 2, 2]}"
            :colors="['#FF8B59', '#AAAAAA', '#AAAAAA']"
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
            :formatter="(input: number) => { return formatBytes(input * 1024) }"
          />
        </div>
      </div>
      <div class="monitor-charts-wrapper">
        <div class="monitor-chart monitor-chart-wide">
          <nodes-monitor-chart
            class="h-full"
            name="procs"
            title="Processes"
            type="area"
            :colors="['#5DA8D1', '#69C197', '#FFB103']"
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
      <div class="grid grid-cols-12 uppercase font-bold select-none">
        <div
          v-for="h in headers"
          @click="() => sortBy(h.id)"
          :key="h.id"
          class="flex flex-row items-center text-center cursor-pointer gap-1 text-xs capitalize hover:text-naturals-N10 transition-colors"
        >
          <span>{{ h.header || h.id }}</span>
          <arrow-down-icon
            class="w-3 h-3"
            :class="{ transform: sortReverse, 'rotate-180': sortReverse }"
            v-if="sort === h.id"
          />
        </div>
      </div>
      <div class="monitor-data-box">
        <div
          class="grid grid-cols-12 text-xs text-naturals-N12 py-2"
          v-for="process in sortedProcesses"
          :key="process.pid"
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
          <div class="col-span-4 truncate">
            {{ process.command }} {{ process.args }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { getContext } from '@/context';
import { ref, onMounted, onUnmounted, computed, Ref } from 'vue';
import { MachineService } from '@/api/talos/machine/machine.pb';
import { Runtime } from "@/api/common/omni.pb";
import { ArrowDownIcon, } from '@heroicons/vue/24/solid';
import { formatBytes } from "@/methods";
import { TalosCPUType, TalosMemoryType, TalosPerfNamespace, TalosCPUID, TalosMemoryID } from '@/api/resources';
import { withContext, withRuntime } from '@/api/options';

import NodesMonitorChart from '@/views/cluster/Nodes/components/NodesMonitorChart.vue';

function diff(a, b) {
  const result: Record<string, number | object> = {};

  for (const key in a) {
    const left = a[key];
    const right = b[key];

    const tleft = typeof left;
    const tright = typeof right;

    if (tleft !== tright) {
      continue;
    }

    if (tleft === "object")
      result[key] = diff(left, right);
    else
      result[key] = left - right;
  }

  return result;
}

const processes: Ref<Record<string, any>[]> = ref([]);
const context = getContext();
const headers = [
  { id: "pid" },
  { id: "state" },
  { id: "threads" },
  { id: "cpu", header: "CPU %" },
  { id: "mem", header: "Memory %" },
  { id: "virtualMemory", header: "Virt Memory" },
  { id: "residentMemory", header: "Res Memory" },
  { id: "cpuTime", header: "Time+" },
  { id: "command" },
];
const sort = ref("cpu");
const sortReverse = ref(true);

let memTotal = 0;
let interval;

const sum = (obj, ...args) => {
  let res = 0;
  for (const k of args) {
    res += obj[k] || 0;
  }

  return res;
};

const getCPUTotal = (stat) => {
  const idle = sum(stat, "idle", "iowait");
  const nonIdle = sum(stat, "user", "nice", "system", "irq", "steal", "softIrq");

  return idle + nonIdle;
};

const prevProcs: Record<string, any> = {};
let prevCPU = 0;

const loadProcs = async () => {
  if (memTotal === 0)
    return;

  const options = [
    withRuntime(Runtime.Talos),
    withContext(context),
  ];

  const resp = await MachineService.Processes({}, ...options);

  const procs: Record<string, any>[] = [];

  const r = await MachineService.SystemStat({}, ...options)

  const systemStat = r.messages![0];
  const cpuTotal = getCPUTotal(systemStat.cpu_total) / systemStat.cpu!.length;

  const total = memTotal * 1024;

  for (const message of resp.messages!) {
    for (const proc of message.processes!) {
      let cpuDiff = 0;

      if (prevProcs[proc.pid!] && proc.cpu_time) {
        cpuDiff = proc.cpu_time! - prevProcs[proc.pid!].cpu_time;
      }

      procs.push({
        mem: parseInt(proc.resident_memory || '0') / total * 100,
        cpu: cpuDiff / (cpuTotal - prevCPU) * 100,
        threads: proc.threads!,
        pid: proc.pid!,
        state: proc.state!,
        virtualMemory: parseInt(proc.virtual_memory || '0'),
        residentMemory: parseInt(proc.resident_memory || '0'),
        command: proc.command!,
        cpuTime: proc.cpu_time || 0,
        args: proc.args!,
      });

      prevProcs[proc.pid!] = proc;
    }
  }

  prevCPU = cpuTotal;

  processes.value = procs;
}

onMounted(() => {
  loadProcs();
  interval = setInterval(loadProcs, 5000);
});

onUnmounted(() => {
  clearInterval(interval);
});

const handleCPU = (oldObj, newObj) => {
  const delta = diff(oldObj, newObj);
  const stat = delta["cpuTotal"];
  const total = getCPUTotal(stat);

  return {
    system: stat["system"] / total * 100,
    user: stat["user"] / total * 100,
  };
};

const handleTotalCPU = (oldObj, newObj) => {
  const point = handleCPU(oldObj, newObj);

  return `${(point.user + point.system).toFixed(1)} %`;
};

const handleMem = (_, m: { used: number, cached: number, buffers: number, total: number }) => {
  const used = m.used - m.cached - m.buffers

  const memoryInitialized = memTotal === 0;

  memTotal = m.total;

  if (memoryInitialized) {
    loadProcs();
  }

  return {
    used: used,
    cached: m.cached,
    buffers: m.buffers,
  }
};

const handleTotalMem = (_, m) => {
  const used = m["used"] - m["cached"] - m["buffers"];

  return `${formatBytes(used * 1024)} / ${formatBytes(m["total"] * 1024)}`;
};

const handleMaxMem = (_, m): number => {
  return m["total"];
};

const handleProcs = (oldObj, newObj) => {
  const delta = diff(oldObj, newObj);

  return {
    created: delta["processCreated"],
    running: newObj["processRunning"],
    blocked: newObj["processBlocked"],
  }
};

const sortedProcesses = computed<Record<string, any>[]>(() => {
  return [...processes.value].sort((a, b) => {
    let res = 0;
    if (a[sort.value] > b[sort.value]) {
      res = 1;
    } else if (a[sort.value] < b[sort.value]) {
      res = -1;
    }

    return sortReverse.value ? -1 * res : res;
  });
});

const sortBy = (id: string) => {
  if (id === sort.value)
    sortReverse.value = !sortReverse.value
  else
    sortReverse.value = true;

  sort.value = id;
};
</script>

<style scoped>
.monitor {
  @apply flex flex-col justify-start pb-5;
}
.monitor-charts-box {
  @apply flex flex-col overflow-hidden;
  padding-bottom: 0 !important;
}
.monitor-charts-wrapper {
  @apply flex flex-1 gap-2 mb-6;
}
.monitor-charts-wrapper:last-of-type {
  @apply mb-0;
}
.monitor-chart {
  @apply flex-1 bg-naturals-N2 rounded p-3 pt-4;
  min-height: 220px;
}
.monitor-chart:nth-child(1) {
  @apply mr-3;
}
.monitor-chart:nth-child(2) {
  @apply ml-3;
}
.monitor-chart-wide {
  @apply border-b border-naturals-N5;
  margin-right: 0 !important;
  padding-bottom: 29px;
  border-radius: 4px 4px 0 0;
}
.monitor-data-wrapper {
  @apply px-2 lg:px-8 pt-5 text-xs text-naturals-N13 flex-1 flex flex-col overflow-hidden w-full bg-naturals-N2;
}
.monitor-data-box {
  @apply flex-1 overflow-x-auto py-3 bg-naturals-N2;
}
</style>
