// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { Userpilot } from 'userpilot'
import { createRouter, createWebHistory } from 'vue-router'
import { handleHotUpdate, routes } from 'vue-router/auto-routes'

import { current } from '@/context'

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  if ('cluster' in to.params) {
    current.value = to.params.cluster
  }

  return true
})

router.afterEach(() => {
  Userpilot.reload()
})

// This will update routes at runtime without reloading the page
if (import.meta.hot) {
  handleHotUpdate(router)
}

export default router
