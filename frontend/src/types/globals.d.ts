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
