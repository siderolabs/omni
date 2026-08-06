// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { dump, load } from 'js-yaml'

import type { VersionContractSpec } from '@/api/omni/specs/virtual.pb'

const patches = new Map(
  Object.entries(
    import.meta.glob('./patches/*.yaml', {
      query: '?raw',
      import: 'default',
    }),
  ).map(([filename, loader]) => [filename.replace(/^\.\/patches\/(.*?)\.yaml$/, '$1'), loader]),
)

export interface Patches {
  kubeSpan: undefined
  hostname: { hostname: string }
  untaintNode: undefined
}

type Patch = keyof Patches

const resolvers: {
  [T in Patch]: (vcSpec: VersionContractSpec) => string
} = {
  kubeSpan: (vcSpec) => (vcSpec.kube_span_multidoc_config ? 'kubeSpanMultiDoc' : 'kubeSpanLegacy'),
  hostname: (vcSpec) =>
    vcSpec.multidoc_network_config_supported ? 'hostnameMultiDoc' : 'hostnameLegacy',
  untaintNode: (vcSpec) =>
    vcSpec.multidoc_kubernetes_config_supported ? 'untaintNodeMultiDoc' : 'untaintNodeLegacy',
}

export async function getPatch<T extends Patch>(
  vcSpec: VersionContractSpec,
  patchName: T,
  ...[patchArgs]: Patches[T] extends undefined ? [patchArgs?: undefined] : [patchArgs: Patches[T]]
) {
  const filename = resolvers[patchName](vcSpec)
  const loader = patches.get(filename)
  if (!loader) throw new Error(`Unknown patch "${filename}"`)

  const raw = await loader()

  if (!patchArgs) return raw

  return dump(substitute(load(raw), patchArgs))
}

function substitute(node: unknown, values: Record<string, unknown>): unknown {
  if (typeof node === 'string') {
    const match = node.match(/^\{\{(\w+)\}\}$/)

    return match ? values[match[1]] : node
  }

  if (Array.isArray(node)) {
    return node.map((item) => substitute(item, values))
  }

  if (node && typeof node === 'object') {
    return Object.fromEntries(
      Object.entries(node).map(([key, value]) => [key, substitute(value, values)]),
    )
  }

  return node
}
