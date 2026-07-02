// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { Runtime } from '@/api/common/omni.pb'
import type { Metadata } from '@/api/v1alpha1/resource.pb'

export interface WatchContext {
  cluster?: string
  node?: string
}

export type WatchOptions = WatchOptionsSingle | WatchOptionsMulti

interface WatchOptionsCommon {
  selectors?: string[]
  selectUsingOR?: boolean
  tailEvents?: number
  offset?: number
  limit?: number
  sortByField?: string
  sortDescending?: boolean
  searchFor?: string[]
  /**
   * Disables watch while true
   */
  skip?: boolean
}

type WatchOptionsBase = WatchOptionsCommon &
  (
    | { runtime: Runtime.Omni; context?: never }
    | { runtime: Exclude<Runtime, Runtime.Omni>; context: WatchContext }
  )

export type WatchOptionsSingle = WatchOptionsBase & {
  resource: Metadata & { id: string }
}

export type WatchOptionsMulti = WatchOptionsBase & {
  resource: Omit<Metadata, 'id'>
}

export const itemID = (item: {
  metadata: { id?: string; name?: string; namespace?: string }
}): string => {
  if (item.metadata === null) {
    return ''
  }

  return `${item.metadata.namespace || 'default'}.${item.metadata.name ?? item.metadata.id}`
}
