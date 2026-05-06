// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { type MaybeRefOrGetter, ref, toValue, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { subscribe } from '@/api/grpc'
import { withAbortController, withContext, withRuntime } from '@/api/options'
import { MachineService, type ServiceEvent } from '@/api/talos/machine/machine.pb'
import type { WatchContext } from '@/api/watch'
import { TCommonStatuses } from '@/constants'

interface Service {
  name?: string
  state?: string
  status: TCommonStatuses
  events?: ServiceEvent[]
}

export function useMachineServices(context: MaybeRefOrGetter<WatchContext>) {
  const services = ref<Service[]>([])
  const serviceListVersion = ref(0)

  watchEffect(async (onCleanup) => {
    // To track for forced updates
    void serviceListVersion.value

    const abortController = new AbortController()
    onCleanup(() => abortController.abort())

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
