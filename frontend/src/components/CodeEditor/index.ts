// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { MONACO_BACKGROUND } from '@/lib/code-theme'
import { defineLazyComponent, runWhenIdle } from '@/lib/lazy-component'

export type { CodeEditorValidator } from './CodeEditor.vue'

const loadCodeEditor = () => import('./CodeEditor.vue')

/**
 * Monaco and monaco-yaml are several MB.
 * Loading it async to not block the whole page till it downloads.
 */
export default defineLazyComponent(loadCodeEditor, {
  placeholderStyle: { backgroundColor: MONACO_BACKGROUND },
})

/**
 * For UI that will show a CodeEditor later, like a closed modal: downloads and sets up Monaco and the config schema for
 * `talosVersion` once the browser is idle, so that work doesn't land in the middle of the modal opening.
 * Repeated calls are cheap, as both are cached.
 */
export function preloadCodeEditor(talosVersion?: string) {
  runWhenIdle(async () => {
    const { preloadConfigSchema } = await loadCodeEditor()

    await preloadConfigSchema(talosVersion)
  })
}
