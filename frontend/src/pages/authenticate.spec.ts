// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test, vi } from 'vitest'
import { render } from 'vitest-browser-vue'
import { ref } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'

import { redirectToURL } from '@/methods/navigate'

import Authenticate from './authenticate.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: { template: '<RouterView />' },
    },
  ],
})

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
  const expectedQueryString = '?thing=%2B123cookies=&bacon=@%23_($%25*%23yes'

  // authenticate.vue reads window.location.search to forward the query string to /login.
  // In browser mode with memory history, window.location is the vitest iframe URL, not
  // the router URL. Use history.pushState to set the expected query string without
  // triggering a page reload.
  window.history.pushState({}, '', expectedQueryString)

  await router.push({ path: '/', query: { thing: '+123cookies=', bacon: '@#_($%*#yes' } })
  await router.isReady()

  await render(Authenticate, {
    global: {
      plugins: [router],
    },
  })

  expect(router.currentRoute.value.fullPath).toBe(`/${expectedQueryString}`)
  expect(vi.mocked(redirectToURL)).toHaveBeenCalledExactlyOnceWith(`/login${expectedQueryString}`)
})
