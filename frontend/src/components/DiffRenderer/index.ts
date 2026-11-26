// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { defineLazyComponent } from '@/lib/lazy-component'

export type { DiffEntry } from './DiffRenderer.vue'

/**
 * `@pierre/diffs` and Shiki are ~168 kB gzipped.
 * Loading it async to not block the whole page till it downloads.
 */
export default defineLazyComponent(() => import('./DiffRenderer.vue'))
