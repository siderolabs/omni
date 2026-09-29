// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

type Maybe<T> = T | undefined
type MaybeArray<T> = T | T[]
type MaybePromise<T> = Promise<T> | T

interface Window {
  monacoConfigured?: boolean
}

/** Design token colours per theme. Set by vite-plugins/theme-init.ts. */
declare const __THEME_COLORS__: Record<
  import('@/methods/theme-core').Theme,
  { page: string; chrome: string }
>
