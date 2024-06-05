// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { Ref } from "vue";
import { Resource } from "../../src/api/grpc";
import { MachineSpec } from "../../src/api/omni/specs/omni.pb";
import { type Event, EventType, ResourceService, WatchRequest, WatchResponse } from "../../src/api/omni/resources/resources.pb";
import Watch from "../../src/api/watch";
import { fetchOption, NotifyStreamEntityArrival, RequestOptions } from "../../src/api/fetch.pb";
import { Runtime } from "../../src/api/common/omni.pb";
import { DefaultNamespace, MachineType } from "../../src/api/resources";
import { ref } from "vue";
import { Metadata } from "../../src/api/v1alpha1/resource.pb";
import { test, describe, expect } from "bun:test";

class fakeStream extends EventTarget {
  private resolve?: (value: void | PromiseLike<void>) => void;
  private reject?: (value: Error | PromiseLike<void>) => void;

  public run(...args: any[]): Promise<void> {
    console.log("starting fake stream", ...args);

    this.dispatchEvent(new Event("run"));

    return new Promise<void>(
      (resolve, reject) => {
        this.resolve = resolve;
        this.reject = reject;
      }
    )
  }

  public waitRunning(timeout: number): Promise<any> {
    return new Promise<any>(
      (resolve, reject) => {
        const onRunning = () => {
          resolve(null);

          this.removeEventListener("run", onRunning);
        };

        setTimeout(() => {
          reject(new Error(`stream is not running after ${timeout}ms`));
          this.removeEventListener("run", onRunning);
        }, timeout);

        this.addEventListener("run", onRunning)
      }
    );
  }

  public close(err?: Error) {
    if (!this.resolve || !this.reject) {
      throw new Error("fake stream is not running");
    }

    err ? this.reject(err) : this.resolve();
  }
}

describe('watch', () => {
  const items: Ref<Resource<MachineSpec>[]> = ref([]);
  const watch = new Watch(items);
  const stream = new fakeStream();

  let callback: NotifyStreamEntityArrival<WatchResponse> | undefined;

  ResourceService.Watch = async (req: WatchRequest, entityNotifier?: NotifyStreamEntityArrival<WatchResponse>, ...options: fetchOption[]): Promise<void> => {
    callback = entityNotifier;

    const opts: RequestOptions = {} as RequestOptions;
    for (const opt of options) {
      opt(opts);
    }

    if (opts?.signal) {
      opts.signal.onabort = () => {
        stream.close();
      };
    }

    await stream.run(opts);
  };

  const pushEvents = (...events: {type: EventType, metadata?: Metadata, resource?: MachineSpec}[]) => {
    if(!callback) {
      throw new Error("the watch is hasn't been started: the callback is null")
    }

    for (const e of events) {
      const event: Event = {
        event_type: e.type,
      };

      if (e.metadata && e.resource) {
        event.resource = JSON.stringify({
          metadata: e.metadata,
          spec: e.resource
        });
      }

      if (e.type === EventType.UPDATED) {
        event.old = JSON.stringify({
          metadata: e.metadata,
          spec: e.resource
        });
      }

      callback({event: event});
    }
  }

  test("event handling", async () => {
    await watch.start({
      runtime: Runtime.Omni,
      resource: {type: MachineType, namespace: DefaultNamespace}
    });

    pushEvents({
      type: EventType.CREATED,
      metadata: {
        id: "1",
        namespace: "default",
        type: MachineType,
      },
      resource: {
        connected: true,
        management_address: "localhost",
      }
    }, {
      type: EventType.CREATED,
      metadata: {
        id: "2",
        namespace: "default",
        type: MachineType,
      },
      resource: {
        connected: true,
        management_address: "localhost",
      }
    }, {
      type: EventType.DESTROYED,
      metadata: {
        id: "2",
        namespace: "default",
        type: MachineType,
      },
      resource: {
        connected: true,
        management_address: "localhost",
      }
    }, {
      type: EventType.UPDATED,
      metadata: {
        id: "1",
        namespace: "default",
        type: MachineType,
      },
      resource: {
        connected: false,
        management_address: "localhost",
      }
    });

    // not yet bootstrapped
    expect(items.value).toHaveLength(0);

    // still loading
    expect(watch.loading.value).toBeTruthy();

    pushEvents({
      type: EventType.BOOTSTRAPPED,
    });

    expect(items.value).toHaveLength(1);

    const machine = items.value[0];
    expect(machine.metadata.id).toBe("1");
    expect(machine.spec.connected).toBeFalsy();
    expect(watch.loading.value).toBeFalsy();

    pushEvents({
      type: EventType.CREATED,
      metadata: {
        id: "2",
        namespace: "default",
        type: MachineType,
      },
      resource: {
        connected: true,
        management_address: "localhost",
      }
    });

    expect(items.value).toHaveLength(2);

    watch.stop();

    expect(items.value).toHaveLength(0);
  });

  test("restarts handling", async () => {
    await watch.start({
      runtime: Runtime.Omni,
      resource: {type: MachineType, namespace: DefaultNamespace},
    });

    const populate = () => {
      pushEvents({
        type: EventType.CREATED,
        metadata: {
          id: "1",
          namespace: "default",
          type: MachineType,
        },
        resource: {
          connected: true,
          management_address: "localhost",
        }
      }, {
        type: EventType.CREATED,
        metadata: {
          id: "2",
          namespace: "default",
          type: MachineType,
        },
        resource: {
          connected: true,
          management_address: "localhost",
        }
      }, {
        type: EventType.CREATED,
        metadata: {
          id: "3",
          namespace: "default",
          type: MachineType,
        },
        resource: {
          connected: true,
          management_address: "localhost",
        }
      }, {
        type: EventType.CREATED,
        metadata: {
          id: "4",
          namespace: "default",
          type: MachineType,
        },
        resource: {
          connected: false,
          management_address: "localhost",
        }
      }, {
        type: EventType.BOOTSTRAPPED,
      });
    }

    populate();

    expect(watch.loading.value).toBeFalsy();
    expect(items.value).toHaveLength(4);

    stream.close(new Error("network error"));

    await stream.waitRunning(6000);

    populate();

    expect(watch.loading.value).toBeFalsy();
    expect(items.value).toHaveLength(4);

    watch.stop();
  });
});
