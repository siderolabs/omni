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

import { useResourceGet } from './useResourceGet'

describe('useResourceGet', () => {
  test('loads a resource and updates state', async () => {
    const resource = {
      metadata: { id: 'res-1', namespace: 'default', type: 'custom.sidero.dev/Resource' },
      spec: { foo: 'bar' },
    }

    server.use(
      http.post('/omni.resources.ResourceService/Get', async ({ request }) => {
        const body = await request.json()
        expect(body).toEqual({
          namespace: 'default',
          type: 'custom.sidero.dev/Resource',
          id: 'res-1',
        })

        return HttpResponse.json({ body: JSON.stringify(resource) })
      }),
    )

    const { data, loading, error } = useResourceGet({
      runtime: Runtime.Omni,
      resource: { namespace: 'default', type: 'custom.sidero.dev/Resource', id: 'res-1' },
    })

    await waitFor(() => expect(data.value).toEqual(resource))
    expect(loading.value).toBe(false)
    expect(error.value).toBeUndefined()
  })

  test('respects skip and only fetches when loadData is invoked', async () => {
    const resource = {
      metadata: { id: 'res-2', namespace: 'default', type: 'custom.sidero.dev/Resource' },
      spec: { foo: 'baz' },
    }

    const handler = vi.fn(() => HttpResponse.json({ body: JSON.stringify(resource) }))

    server.use(http.post('/omni.resources.ResourceService/Get', handler))

    const { data, loading, loadData } = useResourceGet({
      runtime: Runtime.Omni,
      resource: { namespace: 'default', type: 'custom.sidero.dev/Resource', id: 'res-2' },
      skip: true,
    })

    await nextTick()
    expect(handler).not.toHaveBeenCalled()

    const response = await loadData()

    expect(handler).toHaveBeenCalledTimes(1)
    expect(response).toEqual(resource)
    expect(data.value).toEqual(resource)
    expect(loading.value).toBe(false)
  })
})
