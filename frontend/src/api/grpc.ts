// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { ref } from 'vue'

import type { Metadata as TalosMetadata } from '@/api/common/common.pb'
import type { fetchOption, NotifyStreamEntityArrival } from '@/api/fetch.pb'
import { RequestError, setCommonFetchOptions } from '@/api/fetch.pb'
import { Code } from '@/api/google/rpc/code.pb'
import type {
  CreateRequest,
  CreateResponse,
  DeleteRequest,
  DeleteResponse,
  GetRequest,
  ListRequest,
  UpdateRequest,
  UpdateResponse,
  WatchRequest,
  WatchResponse,
} from '@/api/omni/resources/resources.pb'
import { ResourceService as WrappedResourceService } from '@/api/omni/resources/resources.pb'
import { withAbortController, withPathPrefix } from '@/api/options'
import type { Metadata } from '@/api/v1alpha1/resource.pb'

export const initState = () => {
  setCommonFetchOptions(withPathPrefix('/api'))
}

export type RequestContext = {
  cluster: string
  nodes?: string[]
}

export interface StreamingRequest<R, T> {
  (req: T, callback: NotifyStreamEntityArrival<R>, ...options: fetchOption[]): Promise<void>
}

export const subscribe = <R, T>(
  method: StreamingRequest<R, T>,
  params: T,
  handler: NotifyStreamEntityArrival<R>,
  options?: fetchOption[],
  onStart?: () => void,
  onError?: (e: Error) => void,
  onFinish?: () => void,
) => {
  return new Stream(method, params, handler, options, onStart, onError, onFinish)
}

const delay = (value: number): Promise<void> => {
  return new Promise<void>((resolve: (value: void | PromiseLike<void>) => void) => {
    window.setTimeout(resolve, value)
  })
}

export class Stream<R, T> {
  private stopped: boolean = false
  private controller?: AbortController

  public err = ref<Error | null>(null)

  constructor(
    method: StreamingRequest<R, T>,
    params: T,
    handler: NotifyStreamEntityArrival<R>,
    options?: fetchOption[],
    onStart?: () => void,
    onError?: (e: Error) => void,
    onFinish?: () => void,
  ) {
    const opts = options || []
    let currentDelay = 0
    let retryCount = 0

    const run = async () => {
      if (this.stopped) {
        return
      }

      try {
        this.err.value = null
        this.controller = new AbortController()

        const withAbort = opts.concat([withAbortController(this.controller)])

        if (onStart) onStart()

        const callback = (resp: R) => {
          if (!resp) {
            return
          }

          // reset backoff delay if anything got received from the stream
          retryCount = 0

          const err = resp as { metadata?: TalosMetadata; error?: { code: Code; message?: string } }

          if (err.metadata?.error || err.error) {
            const message = err.metadata?.error ?? err.error?.message
            const code = err.error?.code

            if (!code || ![Code.CANCELLED, Code.INTERNAL].includes(code)) {
              this.stopped = true
            }

            const e = new RequestError(message, { code })
            this.err.value = e

            if (onError) {
              onError(e)
            } else {
              console.error('stream error', e)
            }

            return
          }

          handler(resp)
        }

        await method(params, callback, ...withAbort)
      } catch (e) {
        if (this.stopped) {
          return
        }

        console.error('watch failed', e)

        const err = e instanceof Error ? e : new Error(String(e))

        onError?.(err)
        this.err.value = err

        throw err
      }
    }

    ;(async () => {
      while (!this.stopped) {
        try {
          if (currentDelay > 0) {
            await delay(currentDelay)
          }

          await run()

          // break the loop if run ended without any errors
          break
        } catch (e) {
          if (e.code === Code.INVALID_ARGUMENT || e.code === Code.PERMISSION_DENIED) {
            return
          }

          // max delay 10 seconds
          currentDelay = Math.min(((Math.pow(2, retryCount) - 1) / 2) * 1000, 10000)
          // half delay jitter
          currentDelay = currentDelay / 2 + (Math.random() * currentDelay) / 2

          retryCount++
        }
      }

      if (onFinish) {
        onFinish()
      }
    })()
  }

  public shutdown() {
    this.stopped = true
    this.controller?.abort()
  }
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type Resource<T = any, S = any> = {
  metadata: Metadata & { name?: string }
  spec: T
  status?: S
}

// define a wrapper for grpc resource service.
export class ResourceService {
  static async Get<T extends Resource>(request: GetRequest, ...options: fetchOption[]): Promise<T> {
    const res = await WrappedResourceService.Get(request, ...options)

    checkError(res)

    return JSON.parse(res.body || '{}')
  }

  static async List<T extends Resource>(
    request: ListRequest,
    ...options: fetchOption[]
  ): Promise<T[]> {
    const res = await WrappedResourceService.List(request, ...options)

    checkError(res)

    const results: T[] = []

    for (const raw of res.items || []) {
      results.push(JSON.parse(raw))
    }

    return results
  }

  static async Create<T extends Resource>(
    resource: T,
    ...options: fetchOption[]
  ): Promise<CreateResponse> {
    const request: CreateRequest = {
      resource: {
        metadata: resource.metadata,
        spec: JSON.stringify(resource.spec),
      },
    }

    const res = await WrappedResourceService.Create(request, ...options)

    checkError(res)

    return res
  }

  static async Update<T extends Resource>(
    resource: T,
    currentVersion?: string | number,
    ...options: fetchOption[]
  ): Promise<UpdateResponse> {
    resource.metadata.version = (resource.metadata.version || 0)?.toString()

    const request: UpdateRequest = {
      resource: {
        metadata: resource.metadata,
        spec: JSON.stringify(resource.spec),
      },
      currentVersion: currentVersion?.toString() || resource.metadata.version,
    }

    const res = checkError(await WrappedResourceService.Update(request, ...options))

    return res
  }

  static async Delete(request: DeleteRequest, ...options: fetchOption[]): Promise<DeleteResponse> {
    return checkError(await WrappedResourceService.Delete(request, ...options))
  }

  static async Teardown(
    request: DeleteRequest,
    ...options: fetchOption[]
  ): Promise<DeleteResponse> {
    return checkError(await WrappedResourceService.Teardown(request, ...options))
  }

  static async Watch(
    request: WatchRequest,
    callback: NotifyStreamEntityArrival<WatchResponse>,
    options?: fetchOption[],
    onStart?: () => void,
    onError?: (e: Error) => void,
  ): Promise<Stream<WatchRequest, WatchResponse>> {
    return subscribe(WrappedResourceService.Watch, request, callback, options, onStart, onError)
  }
}

export const checkError = <T>(response: { message?: string; code?: Code } & T): T => {
  if (response.code) {
    throw new RequestError(response.message || String(response.code), { code: response.code })
  }

  return response
}
