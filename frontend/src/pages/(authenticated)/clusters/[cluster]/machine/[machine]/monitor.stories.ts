// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createBootstrapEvent, createCreatedEvent, createUpdatedEvent } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { WatchRequest, WatchResponse } from '@/api/omni/resources/resources.pb'
import { TalosCPUType, TalosMemoryType } from '@/api/resources'
import type {
  ProcessesResponse,
  ProcessInfo,
  SystemStatResponse,
} from '@/api/talos/machine/machine.pb'
import type { CPUSpec, MemorySpec } from '@/api/talos/perf.pb'

import NodeMonitor from './monitor.vue'

const meta: Meta<typeof NodeMonitor> = {
  component: NodeMonitor,
}

export default meta
type Story = StoryObj<typeof meta>

const POINTS = 30

// The monitor charts only ever plot UPDATED events (see NodesMonitorChart), and
// the process table stays empty until the memory chart's point-fn observes a
// total — so a bootstrap-only handler would render a blank page. This streams a
// CREATED + BOOTSTRAPPED pair followed by a steady trickle of UPDATED events so
// every chart animates and `memTotal` gets populated. Each watch request gets its
// own stream, so the duplicated CPU watch (CPU + Processes charts) animates too.
function perfStreamHandler<T>(type: string, specAt: (tick: number) => T) {
  const encode = (response: WatchResponse) =>
    new TextEncoder().encode(JSON.stringify(response) + '\n')

  return http.post<never, WatchRequest>(
    '/omni.resources.ResourceService/Watch',
    async ({ request }) => {
      const { type: requestType, namespace, id } = await request.clone().json()

      // Let other watch handlers (or the next matching one) respond.
      if (requestType !== type) return

      const makeResource = (tick: number): Resource<T> => ({
        spec: specAt(tick),
        metadata: {
          namespace,
          type,
          id,
          version: String(tick + 1),
          updated: new Date().toISOString(),
        },
      })

      let intervalId: number
      let tick = 0
      let prev = makeResource(tick)

      const stream = new ReadableStream<Uint8Array>({
        start(controller) {
          controller.enqueue(encode(createCreatedEvent(prev, 1)))
          controller.enqueue(encode(createBootstrapEvent(1)))
          tick = 1

          intervalId = window.setInterval(() => {
            if (tick > POINTS) {
              clearInterval(intervalId)
              return
            }

            const next = makeResource(tick)
            controller.enqueue(encode(createUpdatedEvent(next, prev)))
            prev = next
            tick++
          }, 1000)
        },
        cancel() {
          clearInterval(intervalId)
        },
      })

      return new HttpResponse(stream, {
        headers: {
          'content-type': 'application/json',
          'Grpc-metadata-content-type': 'application/grpc',
        },
      })
    },
  )
}

// Monotonically increasing CPU counters (like /proc/stat) so the chart's
// per-interval deltas stay positive.
const cpuSpecAt = (tick: number): CPUSpec => {
  let user = 0
  let system = 0
  let idle = 0
  let iowait = 0

  // Each interval spends a fixed ~50-unit budget split between busy time
  // (user + system) and idle. `load` oscillates up to ~0.85 so the charted
  // user + system total peaks above 80% during the busy stretches.
  for (let k = 0; k < tick; k++) {
    const load = Math.max(0.05, 0.45 + Math.sin(k / 3) * 0.4)
    const busy = 50 * load

    user += busy * 0.7
    system += busy * 0.3
    iowait += 0.3 + (k % 4) * 0.1
    idle += 50 - busy
  }

  return {
    cpu: Array.from({ length: 8 }, () => ({})),
    cpuTotal: {
      user,
      system,
      idle,
      iowait,
      nice: tick * 0.4,
      irq: tick * 0.05,
      softIrq: tick * 0.05,
      steal: 0,
    },
    processCreated: tick * 13,
    processRunning: 3 + Math.round(Math.abs(Math.sin(tick / 2)) * 5),
    processBlocked: tick % 2,
  }
}

const TOTAL_KIB = 16 * 1024 * 1024 // 16 GiB worth of KiB, matching /proc/meminfo units
const memSpecAt = (tick: number): MemorySpec => ({
  total: TOTAL_KIB,
  used: Math.round((7 + Math.abs(Math.sin(tick / 4)) * 4) * 1024 * 1024),
  cached: Math.round((2 + Math.abs(Math.cos(tick / 6))) * 1024 * 1024),
  buffers: Math.round(0.4 * 1024 * 1024),
})

const PROC_COMMANDS = [
  '/sbin/init',
  '/usr/local/bin/kubelet --config=/etc/kubernetes/kubelet.yaml --kubeconfig=/etc/kubernetes/kubeconfig',
  '/usr/bin/containerd',
  '/usr/local/bin/etcd --data-dir=/var/lib/etcd --advertise-client-urls=https://10.5.0.2:2379',
  'kube-apiserver --advertise-address=10.5.0.2 --secure-port=6443 --etcd-servers=https://127.0.0.1:2379',
  'kube-controller-manager --bind-address=127.0.0.1 --leader-elect=true',
  'kube-scheduler --bind-address=127.0.0.1 --leader-elect=true',
  '/usr/bin/dashboard',
  '/sbin/dhcpcd --quiet --waitip',
  'runc init',
  '/pause',
  '/coredns -conf /etc/coredns/Corefile',
  '/bin/flanneld --ip-masq --kube-subnet-mgr',
  '/usr/local/bin/talosctl-apid',
  '/usr/local/bin/trustd',
]

// Built once at module load so the table is stable across the 5s polling
// interval; only `cpu_time` advances (driven by `processTick`) so CPU% animates.
faker.seed(42)
const baseProcs = faker.helpers.multiple(
  () => {
    const resident = faker.number.int({ min: 1, max: 2048 }) * 1024 * 1024

    return {
      pid: faker.number.int({ min: 1, max: 32768 }),
      state: faker.helpers.arrayElement(['R', 'S', 'D', 'I', 'Z']),
      threads: faker.number.int({ min: 1, max: 64 }),
      virtualMemory: resident * faker.number.int({ min: 2, max: 8 }),
      residentMemory: resident,
      args: faker.helpers.arrayElement(PROC_COMMANDS),
      baseCpuTime: faker.number.float({ min: 0, max: 80000, fractionDigits: 2 }),
      cpuRate: faker.number.float({ min: 0, max: 4.5, fractionDigits: 3 }),
    }
  },
  { count: 36 },
)

let processTick = 0
const processesHandler = http.post('/machine.MachineService/Processes', () => {
  const tick = ++processTick

  return HttpResponse.json({
    messages: [
      {
        metadata: {},
        processes: baseProcs.map((proc): ProcessInfo => {
          // argv[0]: absolute paths stay as-is, bare names get a plausible
          // install path so `executable` mirrors what real Talos reports.
          const argv0 = proc.args.split(' ')[0]
          const executable = argv0.startsWith('/') ? argv0 : `/usr/bin/${argv0}`

          return {
            pid: proc.pid,
            state: proc.state,
            threads: proc.threads,
            virtual_memory: String(proc.virtualMemory),
            resident_memory: String(proc.residentMemory),
            args: proc.args,
            executable,
            command: executable.split('/').pop(),
            cpu_time: Number((proc.baseCpuTime + tick * proc.cpuRate).toFixed(2)),
          }
        }),
      },
    ],
  } satisfies ProcessesResponse)
})

let statTick = 0
const systemStatHandler = http.post('/machine.MachineService/SystemStat', () => {
  const tick = ++statTick

  return HttpResponse.json({
    messages: [
      {
        metadata: {},
        cpu_total: {
          user: tick * 7,
          nice: tick * 0.5,
          system: tick * 2.5,
          idle: tick * 29,
          iowait: tick * 0.4,
          irq: tick * 0.05,
          soft_irq: tick * 0.05,
          steal: 0,
        },
        cpu: Array.from({ length: 8 }, () => ({})),
        process_created: String(tick * 14),
        process_running: '4',
        process_blocked: '1',
      },
    ],
  } satisfies SystemStatResponse)
})

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        perfStreamHandler(TalosCPUType, cpuSpecAt),
        perfStreamHandler(TalosMemoryType, memSpecAt),
        processesHandler,
        systemStatHandler,
      ],
    },
  },
}
