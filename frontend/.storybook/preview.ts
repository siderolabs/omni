// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import '../src/index.css'

import { faker } from '@faker-js/faker'
import { type Preview, setup } from '@storybook/vue3-vite'
import { initialize, mswLoader } from 'msw-storybook-addon'
import { createRouter, createWebHistory, RouterView } from 'vue-router'

// Initialize MSW
initialize({ onUnhandledRequest: 'bypass' })

// Add a blank router
setup((app) => {
  app.use(
    createRouter({
      history: createWebHistory(),
      routes: [{ path: '/:catchAll(.*)', component: RouterView }],
    }),
  )

  // Stub out RouterLink to prevent "Not Found" errors when creating links
  app.component('RouterLink', { template: `<a><slot></slot></a>` })
})

const preview: Preview = {
  beforeEach() {
    faker.seed(0)
  },
  loaders: [mswLoader],
  parameters: {
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },

    a11y: {
      // 'todo' - show a11y violations in the test UI only
      // 'error' - fail CI on a11y violations
      // 'off' - skip a11y checks entirely
      test: 'todo',
    },
  },
}

export default preview
