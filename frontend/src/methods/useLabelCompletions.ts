// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, type MaybeRefOrGetter, toRef, toValue, watch } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { LabelsCompletionSpec } from '@/api/omni/specs/virtual.pb'
import {
  ClusterStatusType,
  LabelsCompletionType,
  MachineStatusType,
  VirtualNamespace,
} from '@/api/resources'
import { getLabelFromID } from '@/methods/labels'
import { useResourceGet } from '@/methods/useResourceGet'

export function useLabelCompletions({
  resourceType,
  filterValue,
}: {
  resourceType: MaybeRefOrGetter<typeof ClusterStatusType | typeof MachineStatusType>
  filterValue: MaybeRefOrGetter<string>
}) {
  const { data: completion, loadData } = useResourceGet<LabelsCompletionSpec>(() => ({
    skip: true,
    runtime: Runtime.Omni,
    resource: {
      id: toValue(resourceType),
      type: LabelsCompletionType,
      namespace: VirtualNamespace,
    },
  }))

  const labelsCompletions = computed(() => {
    const completionEntries = Object.entries(completion.value?.spec.items ?? {})

    return completionEntries.flatMap(([key, { items = [] }]) => {
      const uniqItems = [...new Set(['', ...items])]

      return uniqItems.map((value) => ({ key, value }))
    })
  })

  // we always do completion for the last space separated word
  const match = computed(() => toValue(filterValue).split(' ').at(-1))

  const completions = computed(() => {
    if (!match.value) return []

    const [key, value] = match.value.split(':') as [string, string | undefined]

    return labelsCompletions.value
      .filter((item) =>
        value === undefined
          ? item.key.includes(key) || item.value.includes(key)
          : item.key.includes(key) && item.value.includes(value),
      )
      .map((item) => {
        const label = getLabelFromID(item.key, item.value)

        label.id = item.value === '' ? `has label: ${label.id}` : label.id

        return label
      })
  })

  let abortController: AbortController | null

  watch(toRef(filterValue), async (_, old, onCleanup) => {
    onCleanup(() => abortController?.abort({ reason: 'input changed' }))

    if (old === '' || abortController) {
      abortController = new AbortController()

      await loadData(abortController)

      abortController = null
    }
  })

  return { match, completions }
}
