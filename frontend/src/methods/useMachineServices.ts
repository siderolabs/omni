// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { type MaybeRefOrGetter, ref, toValue, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { subscribe } from '@/api/grpc'
import type { RuntimeContext } from '@/api/options'
import { withAbortController, withContext, withRuntime } from '@/api/options'
import { MachineService, type ServiceEvent } from '@/api/talos/machine/machine.pb'
import { TCommonStatuses } from '@/constants'
import { showError } from '@/notification'

interface Service {
  name?: string
  state?: string
  status: TCommonStatuses
  events?: ServiceEvent[]
}

export function useMachineServices(context: MaybeRefOrGetter<RuntimeContext>) {
  const services = ref<Service[]>([])
  const serviceListVersion = ref(0)

  watchEffect(async (onCleanup) => {
    // To track for forced updates
    void serviceListVersion.value

    const abortController = new AbortController()
    onCleanup(() => abortController.abort())

    try {
      const { messages = [] } = await MachineService.ServiceList(
        {},
        withRuntime(Runtime.Talos),
        withContext(toValue(context)),
        withAbortController(abortController),
      )

      services.value = messages.flatMap(({ services = [] }) =>
        services.map((service) => ({
          name: service.id,
          state: service.state,
          status: service.health?.unknown
            ? TCommonStatuses.HEALTH_UNKNOWN
            : service.health?.healthy
              ? TCommonStatuses.HEALTHY
              : TCommonStatuses.UNHEALTHY,
          events: service.events?.events,
        })),
      )
    } catch (e) {
      if (abortController.signal.aborted) return

      showError('Error', e instanceof Error ? e.message : String(e))
    }
  })

  watchEffect((onCleanup) => {
    const stream = subscribe(
      MachineService.Events,
      {},
      (event) => {
        // For some reason @type is not typed on Any
        const data = event.data as (typeof event.data & { ['@type']?: string }) | undefined

        if (data?.['@type']?.includes('machine.ServiceStateEvent')) {
          // Trigger a services refetch
          serviceListVersion.value++
        }
      },
      [withRuntime(Runtime.Talos), withContext(toValue(context))],
    )

    onCleanup(() => stream.shutdown())
  })

  return { services }
}
