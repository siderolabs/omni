// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useDocumentVisibility, useIntervalFn } from '@vueuse/core'
import { type MaybeRefOrGetter, shallowRef, toValue, watch } from 'vue'

import { ContainerDriver } from '@/api/common/common.pb'
import { Runtime } from '@/api/common/omni.pb'
import { withContext, withRuntime } from '@/api/options'
import { TalosContainersContainerdNamespace } from '@/api/resources'
import { MachineService } from '@/api/talos/machine/machine.pb'
import {
  type ContainerMetrics,
  containerMetrics,
  type StatsSample,
  statsSample,
} from '@/views/Containers/lib/containerStats'

const pollInterval = 5000

/**
 * Polls a machine's container stats while the page is visible. Talos has no watchable resource
 * for them, so this is the one part of the Containers tab that polls.
 */
export function useContainerStats(
  machineId: MaybeRefOrGetter<string>,
  enabled: MaybeRefOrGetter<boolean>,
) {
  const metrics = shallowRef(new Map<string, ContainerMetrics>())
  const visibility = useDocumentVisibility()

  let previous: StatsSample | undefined
  // Bumped whenever polling starts over, so a response to an earlier poll is dropped
  let round = 0

  const startOver = () => {
    previous = undefined
    round++
  }

  const poll = async () => {
    const node = toValue(machineId)
    const pollRound = round

    try {
      const { messages = [] } = await MachineService.Stats(
        { namespace: TalosContainersContainerdNamespace, driver: ContainerDriver.CONTAINERD },
        withRuntime(Runtime.Talos),
        withContext({ node }),
      )

      // Polling paused, or moved to another machine, while the request was out
      if (pollRound !== round) return

      const sample = statsSample(
        messages.flatMap(({ stats = [] }) => stats),
        Date.now(),
      )

      metrics.value = containerMetrics(previous, sample)
      previous = sample
    } catch {
      if (pollRound !== round) return

      // A failed poll shows no metrics rather than stale ones; the next poll tries again
      metrics.value = new Map()
      startOver()
    }
  }

  const { pause, resume } = useIntervalFn(poll, pollInterval, {
    immediate: false,
    immediateCallback: true,
  })

  watch(
    () => [toValue(machineId), toValue(enabled) && visibility.value === 'visible'] as const,
    ([machine, active], old) => {
      if (machine !== old?.[0]) metrics.value = new Map()

      // A gap in polling would average CPU over it, so start over from a fresh sample
      if (machine !== old?.[0] || !active) startOver()

      if (active) resume()
      else pause()
    },
    { immediate: true },
  )

  return { metrics }
}
