// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { MONACO_BACKGROUND } from '@/lib/code-theme'
import { defineLazyComponent } from '@/lib/lazy-component'

export type { CodeEditorValidator } from './CodeEditor.vue'

/**
 * Monaco, monaco-yaml and the Talos config schemas are several MB.
 * Loading it async to not block the whole page till it downloads.
 */
export default defineLazyComponent(() => import('./CodeEditor.vue'), {
  placeholderStyle: { backgroundColor: MONACO_BACKGROUND },
})
