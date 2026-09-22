// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { defineAsyncComponent, h } from 'vue'

import TSpinner from '@/components/Spinner/TSpinner.vue'

export type { DiffEntry } from './DiffRenderer.vue'

/**
 * `@pierre/diffs` and Shiki are ~168 kB gzipped.
 * Loading it async to not block the whole page till it downloads.
 */
export default defineAsyncComponent({
  loader: () => import('./DiffRenderer.vue'),
  loadingComponent: () => h(TSpinner, { class: 'mx-auto my-8 size-6' }),
  delay: 200,
})
