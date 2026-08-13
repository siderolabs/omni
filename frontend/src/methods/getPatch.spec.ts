// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { load } from 'js-yaml'
import { describe, expect, test } from 'vitest'

import type { VersionContractSpec } from '@/api/omni/specs/virtual.pb'

import { getPatch } from './getPatch'

describe('getPatch', () => {
  test('substitutes args into the legacy hostname patch', async () => {
    const vcSpec: VersionContractSpec = { multidoc_network_config_supported: false }

    const patch = await getPatch(vcSpec, 'hostname', { hostname: 'my-node' })

    expect(load(patch)).toEqual({ machine: { network: { hostname: 'my-node' } } })
  })

  test('substitutes args into the multidoc hostname patch', async () => {
    const vcSpec: VersionContractSpec = { multidoc_network_config_supported: true }

    const patch = await getPatch(vcSpec, 'hostname', { hostname: 'my-node' })

    expect(load(patch)).toEqual({
      apiVersion: 'v1alpha1',
      kind: 'HostnameConfig',
      hostname: 'my-node',
      auto: 'off',
    })
  })

  test('resolves the legacy kubeSpan patch', async () => {
    const vcSpec: VersionContractSpec = { kube_span_multidoc_config: false }

    const patch = await getPatch(vcSpec, 'kubeSpan')

    expect(load(patch)).toEqual({
      machine: { network: { kubespan: { enabled: true } } },
      cluster: { discovery: { enabled: true } },
    })
  })

  test('resolves the multidoc kubeSpan patch', async () => {
    const vcSpec: VersionContractSpec = { kube_span_multidoc_config: true }

    const patch = await getPatch(vcSpec, 'kubeSpan')

    expect(load(patch)).toEqual({
      apiVersion: 'v1alpha1',
      kind: 'KubeSpanConfig',
      enabled: true,
    })
  })

  test('resolves the legacy untaintNode patch', async () => {
    const vcSpec: VersionContractSpec = { multidoc_kubernetes_config_supported: false }

    const patch = await getPatch(vcSpec, 'untaintNode')

    expect(load(patch)).toEqual({ cluster: { allowSchedulingOnControlPlanes: true } })
  })

  test('resolves the multidoc untaintNode patch', async () => {
    const vcSpec: VersionContractSpec = { multidoc_kubernetes_config_supported: true }

    const patch = await getPatch(vcSpec, 'untaintNode')

    expect(load(patch)).toEqual({
      apiVersion: 'v1alpha1',
      kind: 'KubeNodeConfig',
      taints: {
        'node-role.kubernetes.io/control-plane': { $patch: 'delete' },
      },
    })
  })
})
