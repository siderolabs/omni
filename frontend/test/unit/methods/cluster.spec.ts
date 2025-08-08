// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { Runtime } from '@/api/common/omni.pb'
import { ResourceService } from '@/api/grpc'
import { withRuntime } from '@/api/options'
import { ClusterType, DefaultNamespace } from '@/api/resources'
import { nextAvailableClusterName } from '@/methods/cluster'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/api/grpc', () => ({
  ResourceService: {
    List: vi.fn(async () => []),
  },
}))

vi.mock('@/api/resources', () => ({
  DefaultNamespace: 'default',
  ClusterType: 'Cluster',
}))

vi.mock('@/api/options', () => ({
  withRuntime: vi.fn((runtime: Runtime) => ({ runtime })),
}))

describe('nextAvailableClusterName', () => {
  const mockListClusters = (clusterIds: string[]) => {
    vi.mocked(ResourceService.List).mockReturnValue(
      Promise.resolve(
        clusterIds.map((id) => ({
          metadata: {
            id,
            type: ClusterType,
            namespace: DefaultNamespace,
            version: '1',
          },
          spec: {},
        })),
      ),
    )
  }

  beforeEach(() => {
    // Clear and reset mocks using type assertion
    vi.mocked(ResourceService.List).mockClear()
    vi.mocked(ResourceService.List).mockImplementation(async () => [])
    vi.mocked(withRuntime).mockClear()
  })

  it('should return the prefix if no clusters exist', async () => {
    mockListClusters([])
    const name = await nextAvailableClusterName('test-cluster')

    expect(name).toBe('test-cluster')
    expect(ResourceService.List).toHaveBeenCalledWith(
      { namespace: DefaultNamespace, type: ClusterType },
      { runtime: Runtime.Omni },
    )
  })

  it("should return the prefix if it's not taken", async () => {
    mockListClusters(['another-cluster', 'yet-another-cluster'])
    const name = await nextAvailableClusterName('test-cluster')

    expect(name).toBe('test-cluster')
  })

  it('should return prefix-1 if prefix is taken and prefix-1 is available', async () => {
    mockListClusters(['test-cluster'])
    const name = await nextAvailableClusterName('test-cluster')

    expect(name).toBe('test-cluster-1')
  })

  it('should return the next available number if prefix and some numbered versions are taken', async () => {
    mockListClusters(['test-cluster', 'test-cluster-1', 'test-cluster-2'])
    const name = await nextAvailableClusterName('test-cluster')

    expect(name).toBe('test-cluster-3')
  })

  it('should handle a different prefix correctly', async () => {
    mockListClusters(['my-cluster', 'my-cluster-1'])
    const name = await nextAvailableClusterName('my-cluster')

    expect(name).toBe('my-cluster-2')
  })

  it('should return prefix-11 if prefix and prefix-1 through prefix-10 are taken', async () => {
    mockListClusters([
      'test-cluster',
      'test-cluster-1',
      'test-cluster-10',
      'test-cluster-2',
      'test-cluster-3',
      'test-cluster-4',
      'test-cluster-5',
      'test-cluster-6',
      'test-cluster-7',
      'test-cluster-8',
      'test-cluster-9',
    ])
    const name = await nextAvailableClusterName('test-cluster')
    expect(name).toBe('test-cluster-11')
  })

  it('should call ResourceService.List with correct parameters and options', async () => {
    mockListClusters([])
    await nextAvailableClusterName('test-prefix')

    expect(ResourceService.List).toHaveBeenCalledTimes(1)
    expect(ResourceService.List).toHaveBeenCalledWith(
      { namespace: DefaultNamespace, type: ClusterType },
      { runtime: Runtime.Omni },
    )
    expect(withRuntime).toHaveBeenCalledWith(Runtime.Omni)
  })

  it('should handle edge case when cluster names have numbers but in a different format', async () => {
    mockListClusters(['test-cluster', 'test-cluster-abc'])
    const name = await nextAvailableClusterName('test-cluster')

    expect(name).toBe('test-cluster-1')
  })
})
