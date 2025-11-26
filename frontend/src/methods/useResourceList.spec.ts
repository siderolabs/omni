// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { server } from '@msw/server'
import { waitFor } from '@testing-library/vue'
import { http, HttpResponse } from 'msw'
import { describe, expect, test, vi } from 'vitest'
import { nextTick } from 'vue'

import { Runtime } from '@/api/common/omni.pb'

import { useResourceList } from './useResourceList'

describe('useResourceList', () => {
  test('loads resources and updates state', async () => {
    const resources = [
      {
        metadata: { id: 'res-1', namespace: 'default', type: 'custom.sidero.dev/Resource' },
        spec: { foo: 'bar' },
      },
      {
        metadata: { id: 'res-2', namespace: 'default', type: 'custom.sidero.dev/Resource' },
        spec: { foo: 'baz' },
      },
    ]

    server.use(
      http.post('/omni.resources.ResourceService/List', async ({ request }) => {
        const body = await request.json()
        expect(body).toEqual({
          namespace: 'default',
          type: 'custom.sidero.dev/Resource',
        })

        return HttpResponse.json({
          items: resources.map((resource) => JSON.stringify(resource)),
          total: resources.length,
        })
      }),
    )

    const { data, loading, error } = useResourceList({
      runtime: Runtime.Omni,
      resource: { namespace: 'default', type: 'custom.sidero.dev/Resource' },
    })

    await waitFor(() => expect(data.value).toEqual(resources))
    expect(loading.value).toBe(false)
    expect(error.value).toBeUndefined()
  })

  test('respects skip and fetches on demand', async () => {
    const resources = [
      {
        metadata: { id: 'res-3', namespace: 'default', type: 'custom.sidero.dev/Resource' },
        spec: { foo: 'qux' },
      },
    ]

    const handler = vi.fn(() =>
      HttpResponse.json({
        items: resources.map((resource) => JSON.stringify(resource)),
        total: resources.length,
      }),
    )

    server.use(http.post('/omni.resources.ResourceService/List', handler))

    const { data, loading, loadData } = useResourceList({
      runtime: Runtime.Omni,
      resource: { namespace: 'default', type: 'custom.sidero.dev/Resource' },
      skip: true,
    })

    await nextTick()
    expect(handler).not.toHaveBeenCalled()

    const response = await loadData()

    expect(handler).toHaveBeenCalledTimes(1)
    expect(response).toEqual(resources)
    expect(data.value).toEqual(resources)
    expect(loading.value).toBe(false)
  })
})
