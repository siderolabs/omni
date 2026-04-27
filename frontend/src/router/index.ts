// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { authGuard } from '@auth0/auth0-vue'
import { Userpilot } from 'userpilot'
import { createRouter, createWebHistory, type RouteRecordRedirect } from 'vue-router'
import { routes } from 'vue-router/auto-routes'

import { AuthFlowQueryParam, FrontendAuthFlow, RedirectQueryParam } from '@/api/resources'
import { AuthType, authType, eulaAccepted } from '@/methods'
import { hasValidKeys } from '@/methods/key'

export type RouteMetaGuard = 'keys' | 'auth0' | 'eula'

declare module 'vue-router' {
  interface RouteMeta {
    guard?: RouteMetaGuard
  }
}

// Redirects for legacy routes
const redirects: RouteRecordRedirect[] = [
  { path: '/omni', redirect: '/' },
  {
    path: '/omni/:catchAll(.*)',
    redirect: (to) => ('catchAll' in to.params ? `/${to.params.catchAll}` : '/'),
  },
  { path: '/cluster', redirect: '/clusters' },
  {
    path: '/cluster/:catchAll(.*)',
    redirect: (to) => ('catchAll' in to.params ? `/clusters/${to.params.catchAll}` : '/'),
  },
  { path: '/machine', redirect: '/machines' },
  {
    path: '/machine/:catchAll(.*)',
    redirect: (to) => ('catchAll' in to.params ? `/machines/${to.params.catchAll}` : '/'),
  },
]

for (const redirect of redirects) {
  routes.push(redirect)
}

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.afterEach(() => {
  Userpilot.reload()
})

router.beforeEach(async (to) => {
  // Check EULA before any other guard — redirect to /eula if not yet accepted.
  if (!eulaAccepted.value && to.name !== 'Eula') {
    return { name: 'Eula' }
  }

  for (const record of to.matched) {
    switch (record.meta.guard) {
      case 'auth0': {
        if (authType.value === AuthType.Auth0) return authGuard(to)
        break
      }
      case 'keys': {
        const authorized = await hasValidKeys()

        if (!authorized) {
          return {
            name: 'Authenticate',
            query: {
              [AuthFlowQueryParam]: FrontendAuthFlow,
              [RedirectQueryParam]: to.fullPath,
            },
          }
        }
        break
      }
    }
  }
})

export default router
