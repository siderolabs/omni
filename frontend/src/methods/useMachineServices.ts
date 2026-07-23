// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { refThrottled } from '@vueuse/core'
import { coerce, gte } from 'semver'
import { type MaybeRefOrGetter, ref, toValue, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { RequestError } from '@/api/fetch.pb'
import { Code } from '@/api/google/rpc/code.pb'
import { subscribe } from '@/api/grpc'
import type { RuntimeContext } from '@/api/options'
import { withAbortController, withContext, withRuntime } from '@/api/options'
import { MachineService, type ServiceEvent } from '@/api/talos/machine/machine.pb'
import { TCommonStatuses } from '@/constants'

export function supportsMaintenanceEvents(maintenance: boolean, talosVersion: string): boolean {
  if (!maintenance) return true

  const version = coerce(talosVersion)

  return version !== null && gte(version, '1.13.0')
}

interface Service {
  name?: string
  state?: string
  status: TCommonStatuses
  events?: ServiceEvent[]
}

export function useMachineServices(
  context: MaybeRefOrGetter<RuntimeContext>,
  supportsEvents: MaybeRefOrGetter<boolean> = true,
) {
  const data = ref<Service[]>([])
  const loading = ref(true)
  const err = ref<Error>()
  const errCode = ref<Code>()

  const serviceListVersion = ref(0)
  const serviceListVersionDebounced = refThrottled(serviceListVersion, 1000)

  let retryCount = 0
  let retryTimer: number | undefined

  watchEffect(async (onCleanup) => {
    // To track for forced updates
    void serviceListVersionDebounced.value

    const abortController = new AbortController()
    onCleanup(() => {
      abortController.abort()
      clearTimeout(retryTimer)
    })

    loading.value = true
    err.value = undefined
    errCode.value = undefined

    try {
      const { messages = [] } = await MachineService.ServiceList(
        {},
        withRuntime(Runtime.Talos),
        withContext(toValue(context)),
        withAbortController(abortController),
      )

      data.value = messages.flatMap(({ services = [] }) =>
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

      retryCount = 0
    } catch (e) {
      if (abortController.signal.aborted) return

      err.value = e instanceof Error ? e : new Error(JSON.stringify(e))
      errCode.value = e instanceof RequestError ? e.code : undefined

      // Retry with backoff
      const backoff = Math.min(2 ** retryCount * 500, 10_000)
      retryCount++
      retryTimer = setTimeout(() => serviceListVersion.value++, backoff)
    } finally {
      loading.value = false
    }
  })

  watchEffect((onCleanup) => {
    if (!toValue(supportsEvents)) return

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

  return { data, loading, err, errCode }
}
