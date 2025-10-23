// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import '../src/index.css'

import { faker } from '@faker-js/faker'
import { type Preview } from '@storybook/vue3-vite'
import { initialize, mswLoader } from 'msw-storybook-addon'
import { sb } from 'storybook/test'
import { vueRouter } from 'storybook-vue3-router'
import { createMemoryHistory } from 'vue-router'

import { routes } from '../src/router/index.ts'

sb.mock('@auth0/auth0-vue')

// Initialize MSW
initialize({ onUnhandledRequest: 'bypass' })

const preview: Preview = {
  beforeEach() {
    faker.seed(0)
  },
  loaders: [mswLoader],
  decorators: [
    vueRouter(undefined, {
      vueRouterOptions: {
        history: createMemoryHistory(),
        routes,
      },
    }),
  ],
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
