// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import {
  inject,
  type InjectionKey,
  type MaybeRefOrGetter,
  provide,
  type ShallowRef,
  toValue,
  watchEffect,
} from 'vue'

interface DeviceNavigation {
  /** Registers the element shown for a device. Returns a function that unregisters it. */
  register(id: string, element: HTMLElement): () => void
  /** Scrolls to the element shown for a device, if there is one. */
  scrollTo(id: string): void
}

const deviceNavigationKey: InjectionKey<DeviceNavigation> = Symbol('DeviceNavigation')

/** Lets the devices shown below this component be scrolled to by ID. */
export function provideDeviceNavigation() {
  const elements = new Map<string, HTMLElement>()

  provide(deviceNavigationKey, {
    register(id, element) {
      elements.set(id, element)

      return () => {
        if (elements.get(id) === element) elements.delete(id)
      }
    },
    scrollTo(id) {
      elements.get(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    },
  })
}

/** Registers `element` as the element shown for the device `id`, for as long as both exist. */
export function useDeviceNavigationTarget(
  id: MaybeRefOrGetter<string>,
  element: Readonly<ShallowRef<HTMLElement | null>>,
) {
  const navigation = inject(deviceNavigationKey, undefined)

  watchEffect((onCleanup) => {
    if (navigation && element.value) onCleanup(navigation.register(toValue(id), element.value))
  })
}

/** Returns a function that scrolls to the element shown for a device. */
export function useScrollToDevice() {
  const navigation = inject(deviceNavigationKey, undefined)

  return (id: string) => navigation?.scrollTo(id)
}
