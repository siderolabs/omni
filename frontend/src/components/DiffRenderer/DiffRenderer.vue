<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
import { registerCustomTheme } from '@pierre/diffs'

import { createOmniShikiTheme, OMNI_CODE_THEME } from '@/lib/code-theme'
import { cn } from '@/methods/utils'

registerCustomTheme(OMNI_CODE_THEME, async () => createOmniShikiTheme())

export interface DiffEntry {
  id: string
  /** Headerless unified diff — just `@@` hunks, no `---`/`+++`/`diff --git`. */
  diff: string
  /** Optional label rendered in the header, in place of the file name. */
  label?: string
}
</script>

<script setup lang="ts">
import {
  CodeView,
  type CodeViewDiffItem,
  type CodeViewOptions,
  parsePatchFiles,
  type PostRenderPhase,
  type SelectionSide,
  setLanguageOverride,
} from '@pierre/diffs'
import { refDebounced, useLocalStorage } from '@vueuse/core'
import { computed, ref, shallowRef, useTemplateRef, watch, watchEffect } from 'vue'

import IconButton from '@/components/Button/IconButton.vue'
import TButtonGroup from '@/components/Button/TButtonGroup.vue'
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import TInput from '@/components/TInput/TInput.vue'

const { diffs, withSearch } = defineProps<{
  diffs: DiffEntry[]
  withSearch?: boolean
}>()

const headerHandlers = new WeakMap<Element, EventListener>()

const viewer = shallowRef<CodeView>()
const scrollContainer = useTemplateRef('scrollContainerRef')
const diffStyle = useLocalStorage<'unified' | 'split'>('diff-renderer-style', 'split')
const wordWrap = useLocalStorage('diff-renderer-wrap', false)

const search = ref('')
const searchDebounced = refDebounced(search, 300)
const currentMatchIndex = ref(0)

const collapsedIds = ref(new Set<string>())

/**
 * CodeView skips deep equality checks on items, so anything that changes on an
 * item — here, `collapsed` — only takes effect if its `version` also changes.
 */
const versions = ref(new Map<string, number>())

const parsedDiffs = computed(() =>
  diffs.flatMap(({ id, diff, label }) => {
    // Omni sends unified diffs without the `--- a/x`/`+++ b/x` preamble. The
    // filename only satisfies the patch format — every diff here is the same
    // machine config, so showing it in the header is noise.
    const patch = `--- config\n+++ config\n${diff}`

    const [parsed] = parsePatchFiles(patch)
    const parsedDiff = parsed?.files[0]

    if (!parsedDiff?.hunks.length) return []

    // The header title is the natural home for the label now that there is no
    // filename to show. The language would normally be inferred from the name,
    // so pin it instead.
    const fileDiff = setLanguageOverride({ ...parsedDiff, name: label ?? '' }, 'yaml')

    return [{ id, fileDiff }]
  }),
)

/**
 * Walks the parsed hunks in render order so matches can be addressed by the
 * line number and side the viewer scrolls and selects by.
 */
const matchedLines = computed(() => {
  if (!withSearch || !searchDebounced.value) return []

  const lowered = searchDebounced.value.toLowerCase()
  const matches: {
    itemId: string
    side: SelectionSide
    lineNumber: number
  }[] = []

  for (const { id, fileDiff } of parsedDiffs.value) {
    const collect = (
      side: SelectionSide,
      lines: string[],
      from: number,
      count: number,
      start: number,
      hunkStartIndex: number,
    ) => {
      for (let i = from; i < from + count; i++) {
        if (lines[i]?.toLowerCase().includes(lowered)) {
          matches.push({ itemId: id, side, lineNumber: start + i - hunkStartIndex })
        }
      }
    }

    for (const hunk of fileDiff.hunks) {
      for (const block of hunk.hunkContent) {
        if (block.type === 'context') {
          collect(
            'additions',
            fileDiff.additionLines,
            block.additionLineIndex,
            block.lines,
            hunk.additionStart,
            hunk.additionLineIndex,
          )

          continue
        }

        collect(
          'deletions',
          fileDiff.deletionLines,
          block.deletionLineIndex,
          block.deletions,
          hunk.deletionStart,
          hunk.deletionLineIndex,
        )

        collect(
          'additions',
          fileDiff.additionLines,
          block.additionLineIndex,
          block.additions,
          hunk.additionStart,
          hunk.additionLineIndex,
        )
      }
    }
  }

  return matches
})

const items = computed(() =>
  parsedDiffs.value.map<CodeViewDiffItem>(({ id, fileDiff }) => ({
    type: 'diff',
    id,
    fileDiff,
    collapsed: collapsedIds.value.has(id),
    version: versions.value.get(id) ?? 0,
  })),
)

watchEffect((onCleanup) => {
  if (!scrollContainer.value) return

  const instance = new CodeView()
  instance.setup(scrollContainer.value)
  viewer.value = instance

  onCleanup(() => instance.cleanUp())
})

watchEffect(() => viewer.value?.setOptions(viewerOptions()))
watchEffect(() => viewer.value?.setItems(items.value))

watch([parsedDiffs, searchDebounced], () => (currentMatchIndex.value = 0))
watch([currentMatchIndex, matchedLines], () => scrollToMatch())

function toggleCollapsed(id: string) {
  const next = new Set(collapsedIds.value)

  if (!next.delete(id)) next.add(id)

  collapsedIds.value = next
  versions.value = new Map(versions.value).set(id, (versions.value.get(id) ?? 0) + 1)
}

function viewerOptions(): CodeViewOptions<undefined, undefined> {
  return {
    theme: OMNI_CODE_THEME,
    themeType: 'dark',
    overflow: wordWrap.value ? 'wrap' : 'scroll',
    diffStyle: diffStyle.value,
    stickyHeaders: true,
    hunkSeparators: 'line-info',
    layout: { paddingTop: 0, paddingBottom: 16, gap: 12 },
    unsafeCSS: '[data-diffs-header] { cursor: pointer; }',
    renderHeaderPrefix: (_fileDiff, context) => renderCollapseToggle(context.item.id),
    onPostRender: (node, _instance, phase, context) =>
      bindHeaderToggle(node, context.item.id, phase),
  }
}

function renderCollapseToggle(id: string) {
  const isCollapsed = collapsedIds.value.has(id)
  const button = document.createElement('button')

  button.type = 'button'
  button.className = cn([
    'grid place-items-center rounded-sm p-0.5 opacity-60 transition hover:opacity-100',
    isCollapsed ? '-rotate-90' : '',
  ])
  button.ariaExpanded = String(!isCollapsed)
  button.ariaLabel = isCollapsed ? 'Expand diff' : 'Collapse diff'
  button.innerHTML = `<svg viewBox="0 0 16 16" width="12" height="12" aria-hidden="true"><path d="M4 6l4 4 4-4" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>`

  // The header itself toggles too, so don't let this bubble into a second flip.
  button.addEventListener('click', (event) => {
    event.stopPropagation()
    toggleCollapsed(id)
  })

  return button
}

function bindHeaderToggle(node: HTMLElement, id: string, phase: PostRenderPhase) {
  const header = node.shadowRoot?.querySelector('[data-diffs-header]')
  if (!header) return

  const existing = headerHandlers.get(header)

  if (existing) {
    header.removeEventListener('click', existing)
    headerHandlers.delete(header)
  }

  if (phase === 'unmount') return

  const handler = () => toggleCollapsed(id)

  header.addEventListener('click', handler)
  headerHandlers.set(header, handler)
}

function scrollToMatch() {
  const match = matchedLines.value[currentMatchIndex.value]
  if (!viewer.value || !match) return

  // A match inside a collapsed file has nothing to scroll to, so open it first.
  if (collapsedIds.value.has(match.itemId)) toggleCollapsed(match.itemId)

  viewer.value.scrollTo({
    type: 'line',
    id: match.itemId,
    lineNumber: match.lineNumber,
    side: match.side,
    align: 'center',
    behavior: 'smooth-auto',
  })

  viewer.value.setSelectedLines({
    id: match.itemId,
    range: {
      start: match.lineNumber,
      end: match.lineNumber,
      side: match.side,
      endSide: match.side,
    },
  })
}

function nextMatch() {
  if (!matchedLines.value.length) return

  currentMatchIndex.value = (currentMatchIndex.value + 1) % matchedLines.value.length
}

function prevMatch() {
  if (!matchedLines.value.length) return

  currentMatchIndex.value =
    (currentMatchIndex.value - 1 + matchedLines.value.length) % matchedLines.value.length
}
</script>

<template>
  <div class="flex h-full flex-col gap-4">
    <div class="flex items-center gap-2">
      <slot
        name="extra-controls"
        :search
        :matched-lines="matchedLines"
        :current-match-index="currentMatchIndex"
        :next-match="nextMatch"
        :prev-match="prevMatch"
      ></slot>

      <template v-if="withSearch">
        <TInput v-model.trim="search" title="Search" class="grow" @keydown.enter="nextMatch" />

        <div class="flex gap-1">
          <IconButton
            :disabled="!matchedLines.length"
            icon="chevron-up"
            aria-label="Previous match"
            @click="prevMatch"
          />
          <IconButton
            :disabled="!matchedLines.length"
            icon="chevron-down"
            aria-label="Next match"
            @click="nextMatch"
          />
        </div>
      </template>

      <TCheckbox v-model="wordWrap" class="shrink-0" label="Wrap" />

      <TButtonGroup
        v-model="diffStyle"
        :options="[
          { label: 'Unified', value: 'unified' },
          { label: 'Split', value: 'split' },
        ]"
      />
    </div>

    <div ref="scrollContainerRef" class="diffs-root min-h-0 flex-1 overflow-y-auto"></div>
  </div>
</template>

<style scoped>
.diffs-root {
  --diffs-font-family: var(--font-mono);
  --diffs-font-size: var(--text-xs);
  --diffs-line-height: var(--leading-relaxed);
  --diffs-header-font-family: var(--font-sans);
  --diffs-tab-size: 2;

  /* Without these the diff tints are derived from the Shiki theme. */
  --diffs-addition-color-override: var(--color-green-g1);
  --diffs-deletion-color-override: var(--color-red-r1);
  --diffs-modified-color-override: var(--color-yellow-y1);

  /* Search match highlight. Blue because green, red and orange are all taken
     by added, removed and changed lines respectively. */
  --diffs-selection-color-override: var(--color-blue-b1);
  --diffs-bg-selection-override: color-mix(in srgb, var(--color-blue-b1) 32%, transparent);
  --diffs-bg-selection-number-override: color-mix(in srgb, var(--color-blue-b1) 55%, transparent);
}
</style>
