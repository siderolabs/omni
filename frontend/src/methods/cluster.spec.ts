// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { beforeEach, describe, expect, it, vi } from 'vitest'

import { Runtime } from '@/api/common/omni.pb'
import { RequestError } from '@/api/fetch.pb'
import { Code } from '@/api/google/rpc/code.pb'
import { ResourceService } from '@/api/grpc'
import { withRuntime } from '@/api/options'
import { ClusterType, DefaultNamespace, MachineInstallDiskConfigType } from '@/api/resources'
import { nextAvailableClusterName, reconcileInstallDiskConfigs } from '@/methods/cluster'

vi.mock('@/api/grpc', () => ({
  ResourceService: {
    List: vi.fn(async () => []),
    Get: vi.fn(async () => ({})),
    Create: vi.fn(async () => ({})),
    Update: vi.fn(async () => ({})),
    Delete: vi.fn(async () => ({})),
  },
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
    expect(ResourceService.List).toHaveBeenCalledExactlyOnceWith(
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
    expect(ResourceService.List).toHaveBeenCalledExactlyOnceWith(
      { namespace: DefaultNamespace, type: ClusterType },
      { runtime: Runtime.Omni },
    )
    expect(withRuntime).toHaveBeenCalledExactlyOnceWith(Runtime.Omni)
  })

  it('should handle edge case when cluster names have numbers but in a different format', async () => {
    mockListClusters(['test-cluster', 'test-cluster-abc'])
    const name = await nextAvailableClusterName('test-cluster')

    expect(name).toBe('test-cluster-1')
  })
})

describe('reconcileInstallDiskConfigs', () => {
  const notFound = () => Object.assign(new RequestError('not found'), { code: Code.NOT_FOUND })

  beforeEach(() => {
    vi.mocked(ResourceService.Get).mockReset()
    vi.mocked(ResourceService.Create).mockReset()
    vi.mocked(ResourceService.Update).mockReset()
    vi.mocked(ResourceService.Delete).mockReset()
  })

  it('creates a config when none exists', async () => {
    vi.mocked(ResourceService.Get).mockRejectedValue(notFound())

    await reconcileInstallDiskConfigs({ 'machine-1': '/dev/sdb' })

    expect(ResourceService.Create).toHaveBeenCalledTimes(1)
    expect(vi.mocked(ResourceService.Create)).toHaveBeenCalledWith(
      {
        metadata: {
          id: 'machine-1',
          namespace: DefaultNamespace,
          type: MachineInstallDiskConfigType,
        },
        spec: { disk: '/dev/sdb' },
      },
      { runtime: Runtime.Omni },
    )
    expect(ResourceService.Update).not.toHaveBeenCalled()
  })

  it('updates an existing config in place, replacing a disk selector', async () => {
    vi.mocked(ResourceService.Get).mockResolvedValue({
      metadata: { id: 'machine-1', version: '3' },
      spec: { disk_selector: 'disk.serial == "old"' },
    })

    await reconcileInstallDiskConfigs({ 'machine-1': '/dev/sdc' })

    expect(ResourceService.Create).not.toHaveBeenCalled()
    expect(ResourceService.Update).toHaveBeenCalledTimes(1)

    expect(vi.mocked(ResourceService.Update)).toHaveBeenCalledWith(
      {
        spec: { disk: '/dev/sdc' },
        metadata: { id: 'machine-1', version: '3' },
      },
      '3',
      { runtime: Runtime.Omni },
    )
  })

  it('deletes on reset to automatic and tolerates a missing config', async () => {
    vi.mocked(ResourceService.Delete).mockRejectedValue(notFound())

    await reconcileInstallDiskConfigs({ 'machine-1': null })

    expect(ResourceService.Delete).toHaveBeenCalledTimes(1)
    expect(ResourceService.Create).not.toHaveBeenCalled()
    expect(ResourceService.Update).not.toHaveBeenCalled()
  })

  it('performs no calls when no selection is pending', async () => {
    await reconcileInstallDiskConfigs({})

    expect(ResourceService.Get).not.toHaveBeenCalled()
    expect(ResourceService.Create).not.toHaveBeenCalled()
    expect(ResourceService.Update).not.toHaveBeenCalled()
    expect(ResourceService.Delete).not.toHaveBeenCalled()
  })

  it('wraps unexpected errors', async () => {
    vi.mocked(ResourceService.Get).mockRejectedValue(
      Object.assign(new RequestError('boom'), { code: Code.INTERNAL }),
    )

    await expect(reconcileInstallDiskConfigs({ 'machine-1': '/dev/sdb' })).rejects.toThrow(
      /The operation failed with the error: boom/,
    )
  })
})
