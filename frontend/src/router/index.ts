// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { Userpilot } from 'userpilot'
import {
  createRouter,
  createWebHistory,
  type RouteRecordRaw,
  type RouteRecordRedirect,
} from 'vue-router'
import { routes } from 'vue-router/auto-routes'

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
  // Cast due to https://github.com/vuejs/router/issues/2656
  ;(routes as RouteRecordRaw[]).push(redirect)
}

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.afterEach(() => {
  Userpilot.reload()
})

export default router
