// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { clusterName } from '@/context'
import type { OmniRequestOptions } from '@/methods/interceptor'

import { Runtime } from './common/omni.pb'
import type { WatchContext } from './watch'

export type GRPCMetadata = Record<string, string | string[]>

const runtimeMap: Record<Runtime, string> = {
  [Runtime.Kubernetes]: 'Kubernetes',
  [Runtime.Talos]: 'Talos',
  [Runtime.Omni]: 'Omni',
}

export const withAbortController = (controller: AbortController) => {
  return (req: OmniRequestOptions) => {
    req.controller = controller
    req.signal = controller.signal
  }
}

export const withPathPrefix = (prefix: string) => {
  return (req: OmniRequestOptions) => {
    if (!req.url.startsWith(prefix)) {
      req.url = `${prefix}${req.url}`
    }
  }
}

export const withRuntime = (runtime: Runtime) => {
  return (req: OmniRequestOptions) => {
    addMetadata(req, { runtime: runtimeMap[runtime] })
  }
}

export const withMetadata = (metadata: GRPCMetadata) => {
  return (req: OmniRequestOptions) => {
    addMetadata(req, metadata)
  }
}

export const withSelectors = (selectors: string[]) => {
  return withMetadata({
    selectors,
  })
}

export const withContext = (context: WatchContext) => {
  return (req: OmniRequestOptions) => {
    const md: GRPCMetadata = {}

    if (context.cluster) {
      md.cluster = context.cluster
    } else {
      const currentContext = clusterName()
      if (currentContext) {
        md.cluster = md.cluster || currentContext
      }
    }

    if (context.node) {
      md.node = context.node
    }

    addMetadata(req, md)
  }
}

export const withTimeout = (timeout: number) => {
  return (req: OmniRequestOptions) => {
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

export const withSkipRequestSignature = () => {
  return (req: OmniRequestOptions) => {
    req.skipSignature = true
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
