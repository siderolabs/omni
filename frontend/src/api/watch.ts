// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { MaybeRefOrGetter, Ref } from 'vue'
import { effectScope, onScopeDispose, ref, toRef, toValue, watch } from 'vue'

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
  private runtime = Runtime.Kubernetes
  private callback: (resp: WatchResponse) => void
  private stream?: Stream<WatchRequest, WatchResponse>

  public readonly loading = ref(false)
  public readonly err = ref<string | null>(null)
  public readonly errCode = ref<Code | null>(null)

  public readonly items?: Ref<T[]>
  public readonly item?: Ref<T | undefined>
  public readonly total = ref(0)

  private watchItems?: WatchItems<T>
  private lastTotal = 0

  constructor(target: Ref<T[]> | Ref<T | undefined>, callback?: Callback<T>) {
    let handler: Callback<T> | undefined

    this.callback = (message: WatchResponse) => {
      const spec: WatchEventSpec<T> = {}

      if (message.event?.resource) spec.res = JSON.parse(message.event.resource)
      if (message.event?.old) spec.old = JSON.parse(message.event.old)

      if (message.event?.event_type === EventType.BOOTSTRAPPED) {
        this.loading.value = false
      }

      callback?.(message, spec)
      handler?.(message, spec)

      if (
        message.total !== undefined ||
        ![EventType.BOOTSTRAPPED, EventType.UNKNOWN].includes(
          message.event?.event_type ?? EventType.UNKNOWN,
        )
      ) {
        this.lastTotal = message.total ?? 0

        if (this.watchItems?.bootstrapped) {
          this.total.value = this.lastTotal
        }
      }
    }

    if (this.isItemsRef(target)) {
      handler = this.listHandler.bind(this)

      this.items = target
    } else {
      handler = this.singleItemHandler.bind(this)

      this.item = target
    }
  }

  public setup(opts: MaybeRefOrGetter<WatchOptions | undefined>) {
    const scope = effectScope()

    scope.run(() => {
      let unmounted = false

      const startWatch = () => {
        this.stop()

        const watchOptions = toValue(opts)

        if (!watchOptions || watchOptions.skip) return

        this.start(watchOptions)

        if (unmounted) {
          this.stop()
        }
      }

      watch(toRef(opts), (newval, oldval) => {
        if (JSON.stringify(newval) !== JSON.stringify(oldval)) {
          startWatch()
        }
      })

      startWatch()

      onScopeDispose(() => {
        unmounted = true

        this.stop()
      })
    })

    return scope
  }

  public start(opts: WatchOptions) {
    if (this.items) {
      this.watchItems = new WatchItems(this.items.value)

      this.items.value.splice(0, this.items.value.length)
    }

    this.watchItems?.setDescending(opts.sortDescending ?? false)

    this.createStream(opts)

    if (!this.stream) {
      return
    }
  }

  public createStream(opts: WatchOptions, onStart?: () => void, onError?: (err: Error) => void) {
    this.loading.value = true
    this.err.value = null
    this.errCode.value = null

    this.runtime = opts.runtime

    const metadata: GRPCMetadata = {}

    if (opts.selectors?.length) {
      metadata.selectors = opts.selectors.join(opts.selectUsingOR ? ';' : ',')
    }

    const fetchOptions: fetchOption[] = [withRuntime(this.runtime), withMetadata(metadata)]

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
      this.callback,
      fetchOptions,
      () => {
        this.onStart()
        if (onStart) onStart()
      },
      (e: Error) => {
        this.onError(e)
        if (onError) onError(e)
      },
    )
  }

  public stop() {
    if (this.stream) {
      this.stream.shutdown()
    }

    if (this.watchItems) {
      this.watchItems.reset()
      this.total.value = 0
    }

    if (this.items) {
      this.items.value.splice(0, this.items.value.length)
    } else if (this.item) {
      this.item.value = undefined
    }
  }

  public id(item: { metadata: { id?: string; name?: string; namespace?: string } }): string {
    return itemID(item)
  }

  private onStart() {
    if (this.watchItems) {
      this.watchItems.reset()
    }

    this.err.value = null
    this.errCode.value = null
    this.loading.value = true
  }

  private onError(error: Error) {
    this.err.value = error.message ?? error.toString()
    this.errCode.value = error instanceof RequestError ? ((error.code as Code) ?? null) : null
    this.loading.value = false
  }

  private isItemsRef(target: Ref<T[]> | Ref<T | undefined>): target is Ref<T[]> {
    return Array.isArray(target.value)
  }

  private singleItemHandler(message: WatchResponse, spec: WatchEventSpec<T>) {
    if (message.event?.event_type === EventType.BOOTSTRAPPED) {
      this.loading.value = false

      return
    }

    if (!spec.res) {
      throw new Error(`malformed ${message.event?.event_type} event: no resource defined`)
    }

    if (!this.item) {
      return
    }

    switch (message.event?.event_type) {
      case EventType.UPDATED:
      case EventType.CREATED:
        this.item.value = spec.res
        break
      case EventType.DESTROYED:
        this.item.value = undefined
        break
    }
  }

  private listHandler(message: WatchResponse, spec: WatchEventSpec<T>) {
    if (!this.items || !this.watchItems) return

    if (message.event?.event_type === EventType.BOOTSTRAPPED) {
      this.loading.value = false

      this.watchItems.bootstrap()

      return
    }

    if (!spec.res) {
      throw new Error(`malformed ${message.event?.event_type} event: no resource defined`)
    }

    switch (message.event?.event_type) {
      case EventType.UPDATED:
      case EventType.CREATED:
        this.watchItems.createOrUpdate(
          { ...spec.res, sortFieldData: message.sort_field_data },
          spec.old,
        )

        break
      case EventType.DESTROYED:
        this.watchItems.remove(itemID(spec.res))

        break
    }
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
class WatchItems<T extends Resource> {
  private items: ResourceSort<T>[]
  private bootstrapList: ResourceSort<T>[] = []

  public bootstrapped: boolean = false
  private sortDescending: boolean = false

  constructor(items: T[]) {
    this.items = items
  }

  public setDescending(descending: boolean) {
    if (this.sortDescending === descending) {
      return
    }

    this.sortDescending = descending
    this.items.sort((a, b): number => {
      return compareFn(a, b, descending)
    })
  }

  public createOrUpdate(item: ResourceSort<T>, old?: T) {
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
      const index = getInsertionIndex(items, item, this.sortDescending)

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
