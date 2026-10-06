// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { Code } from '@/api/google/rpc/code.pb'

const hasValidKeys = vi.fn()
const get = vi.fn()

vi.mock('@/methods/key', () => ({ hasValidKeys: () => hasValidKeys() }))
// the real route table, without the hot reload hook, which has no vite server behind it here
vi.mock('vue-router/auto-routes', async (actual) => ({
  ...(await actual<typeof import('vue-router/auto-routes')>()),
  handleHotUpdate: () => {},
}))
vi.mock('@/api/grpc', () => ({ ResourceService: { Get: () => get() } }))

async function navigateTo(path: string) {
  vi.resetModules()

  const { default: router } = await import('@/router')

  await router.push(path)

  return router.currentRoute.value.name
}

describe('router guard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.history.replaceState({}, '', '/')
  })

  it('sends a signed out user to the sign in page', async () => {
    hasValidKeys.mockResolvedValue(false)

    expect(await navigateTo('/')).toBe('Authenticate')
  })

  it('sends a signed in user to the EULA while it is unaccepted', async () => {
    hasValidKeys.mockResolvedValue(true)
    get.mockRejectedValue({ code: Code.NOT_FOUND })

    expect(await navigateTo('/')).toBe('Eula')
  })

  it('lets a signed in user through once the EULA is accepted', async () => {
    hasValidKeys.mockResolvedValue(true)
    get.mockResolvedValue({})

    expect(await navigateTo('/')).toBe('Home')
  })

  it('lets a user who cannot read the acceptance through', async () => {
    hasValidKeys.mockResolvedValue(true)
    get.mockRejectedValue({ code: Code.PERMISSION_DENIED })

    expect(await navigateTo('/')).toBe('Home')
  })

  // a failed read used to throw out of the guard, which aborted the navigation and left no page at all
  it('lets a user through when the acceptance cannot be read at all', async () => {
    hasValidKeys.mockResolvedValue(true)
    get.mockRejectedValue({ code: Code.UNAUTHENTICATED })

    expect(await navigateTo('/')).toBe('Home')
  })

  it('never holds back the sign out page', async () => {
    hasValidKeys.mockResolvedValue(false)
    get.mockRejectedValue({ code: Code.NOT_FOUND })

    expect(await navigateTo('/logout')).toBe('Logout')
  })
})
