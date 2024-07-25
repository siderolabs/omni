// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { onBeforeUnmount, onMounted, ref, Ref, watch } from 'vue';
import { Runtime } from '@/api/common/omni.pb';
import { EventType, WatchRequest, WatchResponse } from '@/api/omni/resources/resources.pb';
import { Resource, ResourceService, Stream } from '@/api/grpc';
import { Metadata } from '@/api/v1alpha1/resource.pb';
import { withContext, withMetadata, withRuntime, GRPCMetadata } from '@/api/options';
import { fetchOption } from '@/api/fetch.pb';

export interface Callback {
  (message: WatchResponse, spec: WatchEventSpec)
}

export type WatchEventSpec = {
  res?: Resource,
  old?: Resource,
};

export class WatchFunc {
  protected runtime: Runtime = Runtime.Kubernetes;
  protected callback: (resp: WatchResponse) => void;
  protected stream?: Stream<WatchRequest, WatchResponse>;

  public readonly loading: Ref<boolean> = ref(false);
  public readonly running: Ref<boolean> = ref(false);
  public readonly err: Ref<string |  null> = ref(null);

  constructor(callback: Callback) {
    this.callback = (message: WatchResponse) => {
      const spec: WatchEventSpec = {};

      if (message.event?.resource) {
        spec.res = JSON.parse(message.event?.resource);
      }

      if (message.event?.old) {
        spec.old = JSON.parse(message.event?.old);
      }

      if (message.event?.event_type === EventType.BOOTSTRAPPED) {
        this.loading.value = false;
      }

      callback(message, spec);
    };
  }

  // setup is meant to be called from the component setup method
  public setup(opts: WatchSource<WatchOptions | undefined> | WatchOptions) {
    let unmounted = false;

    const reactiveOpts = isRef(opts) ? (opts as WatchSource<WatchOptions>) : null;

    const startWatch = async () => {
      stopWatch();

      const watchOptions = reactiveOpts ? reactiveOpts.value : opts;

      if (!watchOptions)
        return;

      await this.start(watchOptions as WatchOptions);

      if (unmounted) {
        stopWatch();
      }
    };

    if (reactiveOpts) {
      watch(reactiveOpts, (newval, oldval) => {
        if (JSON.stringify(newval) !== JSON.stringify(oldval)) {
          startWatch();
        }
      });
    }

    const stopWatch = async () => {
      this.stop();
    };

    onMounted(async () => {
      await startWatch();
    });

    onBeforeUnmount(async () => {
      unmounted = true;

      stopWatch();
    });
  }

  public start(opts: WatchOptions): Promise<void> {
    return this.createStream(opts);
  }

  public async createStream(opts: WatchOptions, onStart?: () => void, onError?: (err: Error) => void): Promise<void> {
    this.loading.value = true;
    this.err.value = "";

    this.runtime = opts.runtime;

    const metadata: GRPCMetadata = {};

    if (opts.selectors?.length) {
      metadata.selectors = opts.selectors.join(opts.selectUsingOR ? ";" : ",");
    }

    const fetchOptions: fetchOption[] = [
      withRuntime(this.runtime),
      withMetadata(metadata),
    ];

    if (opts.context) {
      fetchOptions.push(
        withContext(opts.context),
      );
    }

    this.stream = await ResourceService.Watch({
      id: opts.resource.id,
      namespace: opts.resource.namespace,
      type: opts.resource.type,
      tail_events: opts.tailEvents,
      limit: opts.limit,
      sort_by_field: opts.sortByField,
      sort_descending: opts.sortDescending,
      search_for: opts.searchFor,
      offset: opts.offset,
    }, this.callback, fetchOptions, () => {
      this.onStart();
      if (onStart) onStart();
    }, (e: Error) => {
      this.onError(e);
      if (onError) onError(e);
    });

    this.running.value = true;
  }

  public stop() {
    this.running.value = false;

    if (this.stream) {
      this.stream.shutdown();
    }
  }

  public id(item: {metadata: {id?: string, name?: string, namespace?: string}}): string {
    return itemID(item);
  }

  protected onStart() {
    this.err.value = null;
    this.loading.value = true;
  }

  protected onError(error: Error) {
    this.err.value = error.message ?? error.toString();
    this.loading.value = false;
  }
}

export type WatchContext = {
  cluster?: string,
  nodes?: string[],
};

export type WatchOptions = {
  runtime: Runtime,
  resource: Metadata,
  context?: WatchContext,
  selectors?: string[],
  selectUsingOR?: boolean,
  tailEvents?: number,
  offset?: number,
  limit?: number,
  sortByField?: string,
  sortDescending?: boolean,
  searchFor?: string[],
}

export default class Watch<T extends Resource> extends WatchFunc {
  public readonly items?: Ref<Resource<T>[]>;
  public readonly item?: Ref<Resource<T> | undefined>;
  public readonly total: Ref<number> = ref(0);

  private watchItems?: WatchItems<T>;
  private lastTotal: number = 0;

  constructor(target: Ref<T[]> | Ref<T | undefined>, callback?: Callback) {
    let handler: Callback | undefined;

    super((event: WatchResponse, spec: WatchEventSpec) => {
      if (callback) {
        callback(event, spec);
      }

      if (handler) {
        handler(event, spec);
      }

      if (event.total !== undefined || ![EventType.BOOTSTRAPPED, EventType.UNKNOWN].includes(event.event?.event_type ?? EventType.UNKNOWN)) {
        this.lastTotal = event.total ?? 0;

        if (this.watchItems?.bootstrapped) {
          this.total.value = this.lastTotal;
        }
      }
    });

    if (target.value && (<Resource<T>[]>target.value).push !== undefined) {
      handler = this.listHandler.bind(this);

      this.items = (<Ref<Resource<T>[]>>target)
    } else {
      handler = this.singleItemHandler.bind(this);

      this.item = (<Ref<Resource<T> | undefined>>target);
    }
  }

  public async start(opts: WatchOptions): Promise<void> {
    if (this.items) {
      this.watchItems = new WatchItems(this.items.value);

      this.items.value.splice(0, this.items.value.length);
    }

    this.watchItems?.setDescending(opts.sortDescending ?? false);

    await this.createStream(opts);

    if (!this.stream) {
      return;
    }

    this.running.value = true;
  }

  public stop() {
    super.stop();

    if (this.watchItems) {
      this.watchItems.reset();
    }

    if (this.items) {
      this.items.value.splice(0, this.items.value.length);
    } else if (this.item) {
      this.item.value = undefined;
    }
  }

  protected onStart() {
    if (this.watchItems) {
      this.watchItems.reset();
    }

    super.onStart();
  }

  private singleItemHandler(message: WatchResponse, spec: WatchEventSpec) {
    if (message.event?.event_type == EventType.BOOTSTRAPPED) {
      this.loading.value = false;

      return;
    }

    if (!spec.res) {
      throw new Error(`malformed ${message.event?.event_type} event: no resource defined`);
    }

    if (!this.item) {
      return;
    }

    switch(message.event?.event_type) {
      case EventType.UPDATED:
      case EventType.CREATED:
        this.item.value = spec.res;
        break;
      case EventType.DESTROYED:
        this.item.value = undefined;
        break;
    }
  }

  private listHandler(message: WatchResponse, spec: WatchEventSpec) {
    if (!this.items || !this.watchItems)
      return;

    if (message.event?.event_type == EventType.BOOTSTRAPPED) {
      this.loading.value = false;

      this.watchItems.bootstrap();

      return;
    }

    if (!spec.res) {
      throw new Error(`malformed ${message.event?.event_type} event: no resource defined`);
    }

    switch(message.event?.event_type) {
      case EventType.UPDATED:
      case EventType.CREATED:
        this.watchItems.createOrUpdate({...spec.res, sortFieldData: message.sort_field_data}, spec.old);

        break;
      case EventType.DESTROYED:
        this.watchItems.remove(itemID(spec.res));

        break;
    }
  }
}

const compareFn = <T>(left: ResourceSort<T>, right: ResourceSort<T>, sortDescending?: boolean): number => {
  const inv = sortDescending ? -1 : 1;

  if (left.sortFieldData && right.sortFieldData) {
    if (left.sortFieldData > right.sortFieldData) {
      return 1 * inv
    } else if (left.sortFieldData < right.sortFieldData) {
      return -1 * inv;
    }
  }

  const leftID = itemID(left);
  const rightID = itemID(right);

  if (leftID > rightID) {
    return 1;
  } else if (leftID < rightID) {
    return -1;
  }

  return 0;
}

function getInsertionIndex<T>(arr: Resource<T>[], item: ResourceSort<T>, sortDescending?: boolean): number {
  const itemsCount = arr.length;

  if (itemsCount === 0) {
    return 0;
  }

  const lastItem = arr[itemsCount - 1];

  if (compareFn(item, lastItem, sortDescending) >= 0) {
    return itemsCount;
  }

  const getMidPoint = (start: number, end: number) => Math.floor((end - start) / 2) + start;
  let start = 0;
  let end = itemsCount - 1;
  let index = getMidPoint(start, end);

  while (start < end) {
    const curItem = arr[index];

    const comparison = compareFn(item, curItem, sortDescending);

    if (comparison === 0) {
      break;
    } else if (comparison < 0) {
      end = index;
    } else {
      start = index + 1;
    }
    index = getMidPoint(start, end);
  }

  return index;
}

export const itemID = (item: {metadata: {id?: string, name?: string, namespace?: string}}): string => {
  if(item.metadata == null) {
    return "";
  }

  return `${item.metadata.namespace || "default"}.${item.metadata.name ?? item.metadata.id}`;
}

export type ResourceSort<T> = Resource<T> & {sortFieldData?: string};

// WatchItems wraps items list and handles sort order, insertions and removals.
class WatchItems<T> {
  private items: ResourceSort<T>[];
  private bootstrapList: ResourceSort<T>[] = [];

  public bootstrapped: boolean = false;
  private sortDescending: boolean = false;

  constructor(items: Resource<T>[]) {
    this.items = items;
  }

  public setDescending(descending: boolean) {
    if (this.sortDescending === descending) {
      return;
    }

    this.sortDescending = descending;
    this.items.sort((a, b): number => {
      return compareFn(a, b, descending);
    })
  }

  public createOrUpdate(item: ResourceSort<T>, old?: Resource<T>) {
    const items = this.bootstrapped ? this.items : this.bootstrapList;

    let foundIndex = this.findIndex(itemID(old ?? item), items);

    if (foundIndex > -1) {
      if (items[foundIndex].sortFieldData !== item.sortFieldData) {
        items.splice(foundIndex, 1);
        foundIndex = -1;
      } else {
        items[foundIndex] = item;
      }
    }

    if (foundIndex < 0) {
      const index = getInsertionIndex(items, item, this.sortDescending);

      items.splice(index, 0, item);
    }
  }

  public remove(id: string) {
    const items = this.bootstrapped ? this.items : this.bootstrapList;

    const foundIndex = this.findIndex(id, items);

    if (foundIndex == -1) {
      return;
    }

    items.splice(foundIndex, 1);
  }

  public reset() {
    this.bootstrapList = [];
    this.bootstrapped = false;
  }

  public bootstrap() {
    this.bootstrapped = true;

    if (this.items) {
      this.items.splice(0, this.items.length);
      this.items.push(...this.bootstrapList);
    }

    this.bootstrapList = [];
  }

  private findIndex(id: string, items: Resource<T>[]): number {
    return items.findIndex((element: Resource<T>) => {
      return itemID(element) === id;
    })
  }
}

export type WatchJoinOptions = WatchOptions & {idFunc?: <T>(res: Resource<T>) => string};

interface WatchSource<T> {
  value: T
}

export class WatchJoin<T extends Resource> {
  private watches: WatchFunc[] = [];
  private items: Ref<Resource<T>[]>;
  private watchItems?: WatchItems<T>;
  private itemMap: Record<string, Record<string, Record<string, ResourceSort<T>>>> = {};
  private primaryResourceType?: string;
  private lastTotal: number = 0;

  public readonly loading = ref(false);
  public readonly err: Ref<string | null> = ref(null);
  public readonly total: Ref<number> = ref(0);

  constructor(items: Ref<Resource<T>[]>) {
    this.items = items;
  }

  public setup(primary: WatchSource<WatchJoinOptions | undefined> | WatchJoinOptions, resources: WatchSource<WatchJoinOptions[] | undefined> | WatchJoinOptions[]) {
    let unmounted = false;

    const reactivePrimary = isRef(primary) ? primary as WatchSource<WatchJoinOptions> : null;
    const reactiveResources = isRef(resources) ? resources as WatchSource<WatchJoinOptions[]> : null;

    const restartIfDiff = (newval: WatchJoinOptions | WatchJoinOptions[], oldval: WatchJoinOptions | WatchJoinOptions[]) => {
      if (JSON.stringify(newval) === JSON.stringify(oldval)) {
        return;
      }

      startWatch();
    }

    const startWatch = async () => {
      stopWatch();

      const primaryOpts: WatchJoinOptions = reactivePrimary ? reactivePrimary.value : primary as WatchJoinOptions;
      const resourcesOpts: WatchJoinOptions[] = reactiveResources ? reactiveResources.value : resources as WatchJoinOptions[];

      if (!primaryOpts || !resourcesOpts)
        return;

      await this.start(primaryOpts, ...resourcesOpts);

      if (unmounted) {
        stopWatch();
      }
    };
    if (reactivePrimary) {
      watch(reactivePrimary as Ref<WatchJoinOptions | WatchJoinOptions[]>, restartIfDiff);
    }

    if (reactiveResources) {
      watch(reactiveResources as Ref<WatchJoinOptions | WatchJoinOptions[]>, restartIfDiff);
    }

    const stopWatch = async () => {
      this.stop();
    };

    onMounted(async () => {
      await startWatch();
    });

    onBeforeUnmount(async () => {
      unmounted = true;

      stopWatch();
    });
  }

  // start initializes the list of watches.
  // the first resource metadata is primary, then it's extended by the specs of the resources defined after.
  public async start(primary: WatchOptions, ...resources: WatchJoinOptions[]): Promise<void> {
    this.stop();

    this.watchItems = new WatchItems(this.items.value);
    this.watchItems.setDescending(primary.sortDescending ?? false);
    this.loading.value = true;
    this.err.value = null;
    this.primaryResourceType = primary.resource.type;

    const handler = (resourceType: string, opts: WatchJoinOptions) => {
      return (resp: WatchResponse, spec: WatchEventSpec) => {
        if (!this.itemMap[resourceType]) {
          this.itemMap[resourceType] = {};
        }

        if (resourceType == this.primaryResourceType) {
          this.lastTotal = resp.total ?? 0;
        }

        if (resourceType == this.primaryResourceType && resp.event?.event_type === EventType.BOOTSTRAPPED) {
          if (!this.watchItems)
            throw new Error("assertion failed on watchItems !== null")

          this.watchItems.bootstrap();
          this.loading.value = false;
          this.total.value = this.lastTotal;

          return;
        }

        if (resourceType == this.primaryResourceType) {
          this.lastTotal = resp.total ?? 0;

          if (this.watchItems?.bootstrapped) {
            this.total.value = this.lastTotal;
          }
        }

        if (!spec.res) {
          return;
        }

        const resourceID = itemID(spec.res);
        const id = opts.idFunc ? `${opts.idFunc(spec.res)}` : resourceID;

        const storeItem = (item: Resource<T>) => {
          if (!this.itemMap[resourceType][id]) {
            this.itemMap[resourceType][id] = {};
          }

          this.itemMap[resourceType][id][resourceID] = {...item, sortFieldData: resp.sort_field_data};
        }

        switch (resp.event?.event_type) {
          case EventType.CREATED:
            storeItem(spec.res);
            break;
          case EventType.UPDATED:
            storeItem(spec.res);
            break;
          case EventType.DESTROYED:
            delete this.itemMap[resourceType][id][resourceID];

            if (Object.keys(this.itemMap[resourceType][id]).length === 0) {
              delete this.itemMap[resourceType][id];
            }

            break;
        }

        this.updateItem(id, resourceID);
      };
    }

    try {
      const list = [primary].concat(resources);
      for (const opts of list) {
        const watch = new WatchFunc(handler(opts.resource.type || "", opts));
        let onStart: (() => void) | undefined;
        let onError: ((err: Error) => void) | undefined;

        if (opts.resource.type === this.primaryResourceType) {
          onStart = () => {
            this.err.value = null;
            this.watchItems?.reset();
          };

          onError = (err: Error) => {
            this.watchItems?.reset();
            this.loading.value = false;
            this.err.value = err.message;
          }
        }

        await watch.createStream(opts, onStart, onError);

        this.watches.push(watch);
      }
    } catch (err) {
      this.stop();

      this.err.value = err.message;

      throw err;
    }
  }

  public stop() {
    for (const w of this.watches) {
      w.stop();
    }

    this.itemMap = {};
    this.loading.value = false;
    this.watchItems?.reset();
  }

  public getRelatedResources(item: Resource<T>, resourceType: string): Record<string, Resource<T>> {
    const id = itemID(item);
    if (!this.itemMap[resourceType]) {
      return {};
    }

    return this.itemMap[resourceType][id] ?? {};
  }

  private updateItem(id: string, resourceID: string) {
    if (!this.watchItems || !this.primaryResourceType || !this.itemMap[this.primaryResourceType]) {
      return;
    }

    const mainGroup = this.itemMap[this.primaryResourceType][id];
    if (!mainGroup) {
      this.watchItems.remove(resourceID);

      return;
    }

    const main = mainGroup[Object.keys(mainGroup)[0]];

    const item: ResourceSort<T> = {
      metadata: main.metadata,
      spec: main.spec,
      sortFieldData: main.sortFieldData,
    };

    for (const key in this.itemMap) {
      const parts = this.itemMap[key][id];
      if (!parts) {
        continue;
      }

      let part: Resource<T> | undefined;
      for (const resourceID in parts) {
        if (!part || (parts[resourceID]?.metadata?.updated || "") > (part?.metadata?.updated || "")) {
          part = parts[resourceID];
        }
      }

      if (!part) {
        continue;
      }

      item.spec = {...item.spec, ...part.spec};

      if (part.status) {
        item.status = {...part.status, ...item.status}
      }

      if (part.metadata.labels) {
        item.metadata.labels = {...item.metadata.labels, ...part.metadata.labels};
      }
    }

    this.watchItems.createOrUpdate(item);
  }
}

const isRef = <T extends object>(value: T | WatchSource<T>) => {
  return "value" in value;
}
