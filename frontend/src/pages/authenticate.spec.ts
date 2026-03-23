// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test, vi } from 'vitest'
import { render } from 'vitest-browser-vue'
import { ref } from 'vue'

import { redirectToURL } from '@/methods/navigate'
import router from '@/router'

import Authenticate from './authenticate.vue'

vi.mock(import('@/methods/navigate'), () => ({
  redirectToURL: vi.fn(),
}))

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

  await render(Authenticate, {
    global: {
      plugins: [router],
    },
  })

  expect(router.currentRoute.value.fullPath).toBe(`/${expectedQueryString}`)
  expect(vi.mocked(redirectToURL)).toHaveBeenCalledExactlyOnceWith(`/login${expectedQueryString}`)
})
