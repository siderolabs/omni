// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useAuth0 } from '@auth0/auth0-vue'
import { server } from '@msw/server'
import { waitFor } from '@testing-library/vue'
import { http, HttpResponse } from 'msw'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import { ref } from 'vue'

import { RequestError } from '@/api/fetch.pb'
import { Code } from '@/api/google/rpc/code.pb'
import { AuthService } from '@/api/omni/auth/auth.pb'
import type { ClusterPermissionsSpec } from '@/api/omni/specs/virtual.pb'
import { ClusterPermissionsType, VirtualNamespace } from '@/api/resources'
import { AuthType, authType } from '@/methods'
import { useClusterPermissions, useLogout } from '@/methods/auth'
import { useIdentity } from '@/methods/identity'
import { useKeys } from '@/methods/key'

vi.mock('@auth0/auth0-vue', () => ({
  useAuth0: vi.fn(),
}))

vi.mock('@/methods/identity')
vi.mock('@/methods/key')
vi.mock('@/api/omni/auth/auth.pb', () => ({
  AuthService: {
    RevokePublicKey: vi.fn(),
  },
}))

describe('useLogout', () => {
  let mockKeys: ReturnType<typeof useKeys>
  let mockIdentity: ReturnType<typeof useIdentity>
  let mockAuth0: {
    logout: ReturnType<typeof vi.fn>
  }
  let originalLocation: Location
  let mockLocation: Location

  beforeEach(() => {
    vi.clearAllMocks()

    mockKeys = {
      publicKeyID: ref('test-key-id'),
      clear: vi.fn(),
      privateKey: ref(undefined),
      privateKeyArmored: ref(undefined),
      publicKey: ref(undefined),
      publicKeyArmored: ref(undefined),
    } as unknown as ReturnType<typeof useKeys>
    vi.mocked(useKeys).mockReturnValue(mockKeys)

    mockIdentity = {
      identity: ref('test-identity'),
      fullname: ref('Test User'),
      avatar: ref('test-avatar'),
      clear: vi.fn(),
    }
    vi.mocked(useIdentity).mockReturnValue(mockIdentity)

    mockAuth0 = {
      logout: vi.fn().mockResolvedValue(undefined),
    }
    vi.mocked(useAuth0).mockReturnValue(mockAuth0 as unknown as ReturnType<typeof useAuth0>)

    originalLocation = window.location
    mockLocation = {
      ...originalLocation,
      href: 'http://localhost:3000',
      origin: 'http://localhost:3000',
    } as Location
    delete (window as { location?: Location }).location
    Object.defineProperty(window, 'location', {
      value: mockLocation,
      writable: true,
      configurable: true,
    })

    Object.defineProperty(window, 'top', {
      value: window,
      writable: true,
      configurable: true,
    })
  })

  afterEach(() => {
    Object.defineProperty(window, 'location', {
      value: originalLocation,
      writable: true,
      configurable: true,
    })
    vi.clearAllMocks()
  })

  test('should revoke public key and clear keys and identity when publicKeyID exists', async () => {
    vi.mocked(AuthService.RevokePublicKey).mockResolvedValue(
      {} as Awaited<ReturnType<typeof AuthService.RevokePublicKey>>,
    )

    const logout = useLogout()
    await logout()

    expect(AuthService.RevokePublicKey).toHaveBeenCalledWith({
      public_key_id: 'test-key-id',
    })
    expect(mockKeys.clear).toHaveBeenCalled()
    expect(mockIdentity.clear).toHaveBeenCalled()
  })

  test('should not revoke public key when publicKeyID is falsy', async () => {
    mockKeys.publicKeyID.value = ''

    const logout = useLogout()
    await logout()

    expect(AuthService.RevokePublicKey).not.toHaveBeenCalled()
  })

  test('should not throw when RevokePublicKey fails with UNAUTHENTICATED error', async () => {
    const error = new RequestError('Unauthenticated')
    error.code = Code.UNAUTHENTICATED
    vi.mocked(AuthService.RevokePublicKey).mockRejectedValue(error)

    const logout = useLogout()
    await expect(logout()).resolves.not.toThrow()

    expect(AuthService.RevokePublicKey).toHaveBeenCalled()
    expect(mockKeys.clear).toHaveBeenCalled()
    expect(mockIdentity.clear).toHaveBeenCalled()
  })

  test('should throw when RevokePublicKey fails with non-UNAUTHENTICATED error', async () => {
    const error = new Error('Server error') as Error & { code: Code }
    error.code = Code.INTERNAL
    vi.mocked(AuthService.RevokePublicKey).mockRejectedValue(error)

    const logout = useLogout()
    await expect(logout()).rejects.toThrow('Server error')

    expect(AuthService.RevokePublicKey).toHaveBeenCalled()
    expect(mockKeys.clear).not.toHaveBeenCalled()
    expect(mockIdentity.clear).not.toHaveBeenCalled()
  })

  test('should call auth0.logout when authType is Auth0', async () => {
    vi.mocked(AuthService.RevokePublicKey).mockResolvedValue(
      {} as Awaited<ReturnType<typeof AuthService.RevokePublicKey>>,
    )
    authType.value = AuthType.Auth0

    const logout = useLogout()
    await logout()

    expect(mockAuth0.logout).toHaveBeenCalledWith({
      logoutParams: {
        returnTo: 'http://localhost:3000',
      },
    })
    expect(mockKeys.clear).toHaveBeenCalled()
    expect(mockIdentity.clear).toHaveBeenCalled()
  })

  test.each([AuthType.SAML, AuthType.OIDC, AuthType.None])(
    'should redirect to logout URL when authType is %s',
    async (mockAuthType) => {
      vi.mocked(AuthService.RevokePublicKey).mockResolvedValue(
        {} as Awaited<ReturnType<typeof AuthService.RevokePublicKey>>,
      )
      authType.value = mockAuthType

      const logout = useLogout()
      await logout()

      expect(mockAuth0.logout).not.toHaveBeenCalled()
      expect(window.location.href).toBe('/logout?flow=frontend')
      expect(mockKeys.clear).toHaveBeenCalled()
      expect(mockIdentity.clear).toHaveBeenCalled()
    },
  )
})

describe('useClusterPermissions', () => {
  function makeHandler(spec: Partial<ClusterPermissionsSpec>, clusterName?: string) {
    return http.post('/omni.resources.ResourceService/Get', async ({ request }) => {
      const body = (await request.json()) as { id: string; type: string }
      if (body.type !== ClusterPermissionsType) return
      if (clusterName !== undefined && body.id !== clusterName) return
      return HttpResponse.json({
        body: JSON.stringify({
          metadata: { id: body.id, namespace: VirtualNamespace, type: ClusterPermissionsType },
          spec,
        }),
      })
    })
  }

  test('defaults to false before data loads', () => {
    const { canUpdateKubernetes, canManageConfigPatches } = useClusterPermissions('perm-defaults')
    expect(canUpdateKubernetes.value).toBe(false)
    expect(canManageConfigPatches.value).toBe(false)
  })

  test('reflects loaded spec fields', async () => {
    server.use(makeHandler({ can_update_kubernetes: true, can_update_talos: false }))
    const { canUpdateKubernetes, canUpdateTalos } = useClusterPermissions('perm-fields')
    await waitFor(() => expect(canUpdateKubernetes.value).toBe(true))
    expect(canUpdateTalos.value).toBe(false)
  })

  test('caches scope per cluster key — only one fetch per name', async () => {
    const handler = vi.fn(() =>
      HttpResponse.json({
        body: JSON.stringify({
          metadata: {
            id: 'perm-cached',
            namespace: VirtualNamespace,
            type: ClusterPermissionsType,
          },
          spec: { can_update_kubernetes: true },
        }),
      }),
    )
    server.use(http.post('/omni.resources.ResourceService/Get', handler))

    const { canUpdateKubernetes: a } = useClusterPermissions('perm-cached')
    const { canUpdateKubernetes: b } = useClusterPermissions('perm-cached')

    await waitFor(() => expect(a.value).toBe(true))
    expect(b.value).toBe(true)
    expect(handler).toHaveBeenCalledTimes(1)
  })

  test('reacts when cluster ref changes', async () => {
    const specByCluster: Record<string, Partial<ClusterPermissionsSpec>> = {
      'perm-cluster-a': { can_update_kubernetes: true, can_update_talos: false },
      'perm-cluster-b': { can_update_kubernetes: false, can_update_talos: true },
    }

    // Single handler — two chained handlers can't both read the request body stream.
    server.use(
      http.post('/omni.resources.ResourceService/Get', async ({ request }) => {
        const body = (await request.json()) as { id: string; type: string }
        if (body.type !== ClusterPermissionsType) return
        const spec = specByCluster[body.id]
        if (!spec) return
        return HttpResponse.json({
          body: JSON.stringify({
            metadata: { id: body.id, namespace: VirtualNamespace, type: ClusterPermissionsType },
            spec,
          }),
        })
      }),
    )

    const cluster = ref('perm-cluster-a')
    const { canUpdateKubernetes, canUpdateTalos } = useClusterPermissions(cluster)

    await waitFor(() => expect(canUpdateKubernetes.value).toBe(true))
    expect(canUpdateTalos.value).toBe(false)

    cluster.value = 'perm-cluster-b'

    await waitFor(() => expect(canUpdateTalos.value).toBe(true))
    expect(canUpdateKubernetes.value).toBe(false)
  })
})
