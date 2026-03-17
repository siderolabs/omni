// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { render } from '@testing-library/vue'
import { expect, test, vi } from 'vitest'
import { ref } from 'vue'

import router from '@/router'

import Authenticate from './authenticate.vue'

vi.mock(import('@/methods'), async (importOriginal) => {
  const original = await importOriginal()

  return {
    ...original,
    authType: ref(original.AuthType.SAML),
  }
})

test('Forwards query string for SAML auth', async () => {
  await router.push({ path: '/', query: { thing: '+123cookies=', bacon: '@#_($%*#yes' } })
  await router.isReady()

  const expectedQueryString = '?thing=%2B123cookies=&bacon=@%23_($%25*%23yes'

  // Mock window.location to prevent navigation errors
  const mockLocation = {
    href: 'http://localhost:3000/',
    origin: 'http://localhost:3000',
    search: expectedQueryString,
  }

  Object.defineProperty(window, 'location', {
    value: mockLocation,
    writable: true,
  })

  // Spy on the href setter to verify the redirect URL
  const locationHrefSpy = vi.spyOn(mockLocation, 'href', 'set')

  render(Authenticate, {
    global: {
      plugins: [router],
    },
  })

  expect(router.currentRoute.value.fullPath).toBe(`/${expectedQueryString}`)
  expect(locationHrefSpy).toHaveBeenCalledExactlyOnceWith(`/login${expectedQueryString}`)
})
