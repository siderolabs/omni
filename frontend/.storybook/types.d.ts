// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import '@storybook/vue3-vite'

import type { MswParameters } from 'msw-storybook-addon'

declare module '@storybook/vue3-vite' {
  // eslint-disable-next-line @typescript-eslint/no-empty-object-type
  interface Parameters extends MswParameters {}
}
