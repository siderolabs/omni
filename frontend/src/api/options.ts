// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { clusterName } from '@/context'

import { Runtime } from './common/omni.pb'
import type { RequestOptions } from './fetch.pb'
import type { WatchContext } from './watch'

export type GRPCMetadata = Record<string, string | string[]>

const runtimeMap: Record<Runtime, string> = {
  [Runtime.Kubernetes]: 'Kubernetes',
  [Runtime.Talos]: 'Talos',
  [Runtime.Omni]: 'Omni',
}

export const withAbortController = (controller: AbortController) => {
  return (req: RequestOptions) => {
    req.controller = controller
    req.signal = controller.signal
  }
}

export const withPathPrefix = (prefix: string) => {
  return (req: RequestOptions) => {
    if (!req.url.startsWith(prefix)) {
      req.url = `${prefix}${req.url}`
    }
  }
}

export const withRuntime = (runtime: Runtime) => {
  return (req: RequestOptions) => {
    addMetadata(req, { runtime: runtimeMap[runtime] })
  }
}

export const withMetadata = (metadata: GRPCMetadata) => {
  return (req: RequestOptions) => {
    addMetadata(req, metadata)
  }
}

export const withSelectors = (selectors: string[]) => {
  return withMetadata({
    selectors,
  })
}

export const withContext = (context: WatchContext) => {
  return (req: RequestOptions) => {
    const md: GRPCMetadata = {}

    if (context.cluster) {
      md.cluster = context.cluster
    } else {
      const currentContext = clusterName()
      if (currentContext) {
        md.cluster = md.cluster || currentContext
      }
    }

    if (context.nodes) {
      md.nodes = context.nodes
    }

    addMetadata(req, md)
  }
}

export const withTimeout = (timeout: number) => {
  return (req: RequestOptions) => {
    if (!req.controller) {
      const controller = new AbortController()
      req.signal = controller.signal
      req.controller = controller
    }

    window.setTimeout(() => {
      req.controller?.abort()
    }, timeout)
  }
}

const addMetadata = (req: RequestInit, headers: GRPCMetadata) => {
  if (!req.headers) {
    req.headers = new Headers()
  }

  const h = req.headers as Headers

  for (const id in headers) {
    h.append(`Grpc-Metadata-${id}`, headers[id].toString())
  }
}
