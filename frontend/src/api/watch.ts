// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Ref } from 'vue'
import { ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { type fetchOption, RequestError } from '@/api/fetch.pb'
import type { Code } from '@/api/google/rpc/code.pb'
import type { Resource, Stream } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { WatchRequest, WatchResponse } from '@/api/omni/resources/resources.pb'
import { EventType } from '@/api/omni/resources/resources.pb'
import type { GRPCMetadata } from '@/api/options'
import { withContext, withMetadata, withRuntime } from '@/api/options'
import type { Metadata } from '@/api/v1alpha1/resource.pb'

export interface Callback<T extends Resource> {
  (message: WatchResponse, spec: WatchEventSpec<T>): void
}

interface WatchEventSpec<T extends Resource> {
  res?: T
  old?: T
}

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

export default class Watch<T extends Resource> {
  private stream?: Stream<WatchRequest, WatchResponse>

  public readonly err = ref<string | null>(null)
  public readonly errCode = ref<Code | null>(null)

  constructor(
    private readonly loading: Ref<boolean>,
    private readonly callback?: Callback<T>,
    private readonly onStart?: () => void,
    private readonly onError?: (e: Error) => void,
  ) {}

  public start(opts: WatchOptions) {
    this.loading.value = true
    this.err.value = null
    this.errCode.value = null

    const metadata: GRPCMetadata = {}

    if (opts.selectors?.length) {
      metadata.selectors = opts.selectors.join(opts.selectUsingOR ? ';' : ',')
    }

    const fetchOptions: fetchOption[] = [withRuntime(opts.runtime), withMetadata(metadata)]

    if (opts.context) {
      fetchOptions.push(withContext(opts.context))
    }

    this.stream = ResourceService.Watch(
      {
        id: 'id' in opts.resource ? opts.resource.id : undefined,
        namespace: opts.resource.namespace,
        type: opts.resource.type,
        tail_events: opts.tailEvents,
        limit: opts.limit,
        sort_by_field: opts.sortByField,
        sort_descending: opts.sortDescending,
        search_for: opts.searchFor,
        offset: opts.offset,
      },
      (message) => {
        const spec: WatchEventSpec<T> = {}

        if (message.event?.resource) spec.res = JSON.parse(message.event.resource)
        if (message.event?.old) spec.old = JSON.parse(message.event.old)

        if (message.event?.event_type === EventType.BOOTSTRAPPED) {
          this.loading.value = false
        }

        this.callback?.(message, spec)
      },
      fetchOptions,
      () => {
        this.onStart?.()

        this.err.value = null
        this.errCode.value = null
        this.loading.value = true
      },
      (error) => {
        this.onError?.(error)

        this.err.value = error.message ?? error.toString()
        this.errCode.value = error instanceof RequestError ? ((error.code as Code) ?? null) : null
        this.loading.value = false
      },
    )
  }

  public stop() {
    this.stream?.shutdown()
  }
}

const compareFn = <T extends Resource>(
  left: ResourceSort<T>,
  right: ResourceSort<T>,
  sortDescending?: boolean,
): number => {
  const inv = sortDescending ? -1 : 1

  if (left.sortFieldData && right.sortFieldData) {
    if (left.sortFieldData > right.sortFieldData) {
      return 1 * inv
    } else if (left.sortFieldData < right.sortFieldData) {
      return -1 * inv
    }
  }

  const leftID = itemID(left)
  const rightID = itemID(right)

  if (leftID > rightID) {
    return 1
  } else if (leftID < rightID) {
    return -1
  }

  return 0
}

function getInsertionIndex<T extends Resource>(
  arr: T[],
  item: ResourceSort<T>,
  sortDescending?: boolean,
): number {
  const itemsCount = arr.length

  if (itemsCount === 0) {
    return 0
  }

  const lastItem = arr[itemsCount - 1]

  if (compareFn(item, lastItem, sortDescending) >= 0) {
    return itemsCount
  }

  const getMidPoint = (start: number, end: number) => Math.floor((end - start) / 2) + start
  let start = 0
  let end = itemsCount - 1
  let index = getMidPoint(start, end)

  while (start < end) {
    const curItem = arr[index]

    const comparison = compareFn(item, curItem, sortDescending)

    if (comparison === 0) {
      break
    } else if (comparison < 0) {
      end = index
    } else {
      start = index + 1
    }
    index = getMidPoint(start, end)
  }

  return index
}

export const itemID = (item: {
  metadata: { id?: string; name?: string; namespace?: string }
}): string => {
  if (item.metadata === null) {
    return ''
  }

  return `${item.metadata.namespace || 'default'}.${item.metadata.name ?? item.metadata.id}`
}

type ResourceSort<T> = T & { sortFieldData?: string }

// WatchItems wraps items list and handles sort order, insertions and removals.
export class WatchItems<T extends Resource> {
  private items: ResourceSort<T>[]
  private bootstrapList: ResourceSort<T>[] = []

  public bootstrapped: boolean = false

  constructor(items: T[]) {
    this.items = items
  }

  public createOrUpdate(item: ResourceSort<T>, sortDescending: boolean, old?: T) {
    const items = this.bootstrapped ? this.items : this.bootstrapList

    let foundIndex = this.findIndex(itemID(old ?? item), items)

    if (foundIndex > -1) {
      if (items[foundIndex].sortFieldData !== item.sortFieldData) {
        items.splice(foundIndex, 1)
        foundIndex = -1
      } else {
        items[foundIndex] = item
      }
    }

    if (foundIndex < 0) {
      const index = getInsertionIndex(items, item, sortDescending)

      items.splice(index, 0, item)
    }
  }

  public remove(id: string) {
    const items = this.bootstrapped ? this.items : this.bootstrapList

    const foundIndex = this.findIndex(id, items)

    if (foundIndex === -1) {
      return
    }

    items.splice(foundIndex, 1)
  }

  public reset() {
    this.bootstrapList = []
    this.bootstrapped = false
  }

  public bootstrap() {
    this.bootstrapped = true

    if (this.items) {
      this.items.splice(0, this.items.length)
      this.items.push(...this.bootstrapList)
    }

    this.bootstrapList = []
  }

  private findIndex(id: string, items: T[]): number {
    return items.findIndex((element: T) => {
      return itemID(element) === id
    })
  }
}
