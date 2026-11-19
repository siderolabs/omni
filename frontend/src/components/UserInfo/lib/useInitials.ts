// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, type MaybeRefOrGetter, toValue } from 'vue'

export function useInitials(source: MaybeRefOrGetter<string>) {
  return computed(() => {
    const segmenter = new Intl.Segmenter()

    const letters = toValue(source)
      // Sometimes email is used as the source, so always check for @
      .split('@')[0]
      .split(/[\s._-]+/)
      .filter(Boolean)
      .map((word) => {
        const [first] = segmenter.segment(word)
        return first.segment
      })
      // Drop non-letters, fail-safe
      .filter((l) => /^\p{L}/u.test(l))

    const initials =
      letters.length > 1 ? [letters[0], letters.at(-1)!].join('') : (letters[0] ?? '')

    return initials.toUpperCase()
  })
}
