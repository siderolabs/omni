// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Stat } from '@/api/talos/machine/machine.pb'

/** What Talos reports one container instance using, as the table shows it. */
export interface ContainerMetrics {
  /** Share of one CPU core over the last poll, in percent. Absent until there are two samples. */
  cpuPercent?: number
  /** Memory in use, in bytes. */
  memory?: number
}

/** One poll of `MachineService.Stats`, keyed by containerd ID. */
export interface StatsSample {
  /** When the sample was taken, in milliseconds. */
  at: number
  /** CPU time used since the instance started, in nanoseconds. */
  cpu: Map<string, bigint>
  memory: Map<string, number>
}

/**
 * The containerd ID Talos runs a container's instance under, which is what Stats is keyed by.
 * Every restart runs under a new generation, so a new ID.
 */
export const statsKey = (id: string, generation = 0) => `${id}-${generation}`

export function statsSample(stats: Stat[], at: number): StatsSample {
  const cpu = new Map<string, bigint>()
  const memory = new Map<string, number>()

  for (const { id, cpu_usage, memory_usage } of stats) {
    if (!id) continue

    if (cpu_usage) cpu.set(id, BigInt(cpu_usage))
    if (memory_usage) memory.set(id, Number(memory_usage))
  }

  return { at, cpu, memory }
}

/**
 * Metrics per containerd ID from two consecutive samples. CPU time is a running total, so a
 * percentage needs the previous sample: an instance seen for the first time, or whose counter went
 * back, gets none rather than a spike.
 */
export function containerMetrics(previous: StatsSample | undefined, current: StatsSample) {
  const elapsedNs = previous ? (current.at - previous.at) * 1e6 : 0
  const metrics = new Map<string, ContainerMetrics>()

  for (const id of new Set([...current.cpu.keys(), ...current.memory.keys()])) {
    const now = current.cpu.get(id)
    const before = previous?.cpu.get(id)

    metrics.set(id, {
      cpuPercent:
        elapsedNs > 0 && now !== undefined && before !== undefined && now >= before
          ? (Number(now - before) / elapsedNs) * 100
          : undefined,
      memory: current.memory.get(id),
    })
  }

  return metrics
}

/** CPU as the table shows it: one decimal under 10%, whole numbers from there. */
export function formatCPU(percent: number) {
  const tenths = Math.round(percent * 10) / 10

  return `${tenths < 10 ? tenths.toFixed(1) : Math.round(percent)}%`
}
