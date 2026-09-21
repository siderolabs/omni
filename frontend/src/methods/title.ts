// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import {
  computed,
  type MaybeRefOrGetter,
  onScopeDispose,
  shallowReactive,
  toValue,
  watchEffect,
} from 'vue'

/**
 * Stack of page titles
 */
const stack = shallowReactive<(() => Maybe<string>[])[]>([])

/**
 * Flattened title stack, with the most-specific part first.
 */
const documentTitle = computed(() =>
  stack
    .flatMap((entry) => entry())
    .filter(Boolean)
    .reverse()
    .concat('Omni')
    .join(' · '),
)

// Actual updating of the document title
watchEffect(() => (document.title = documentTitle.value))

/**
 * Contribute one or more segments to the document title for as long as the
 * calling component is mounted. Segments may be undefined while the data
 * backing them is still loading, in which case they are left out.
 */
export function useTitle(title: MaybeRefOrGetter<MaybeArray<Maybe<string>>>) {
  const entry = () => {
    const segments = toValue(title)

    return Array.isArray(segments) ? segments : [segments]
  }

  stack.push(entry)

  onScopeDispose(() => stack.splice(stack.indexOf(entry), 1))
}
