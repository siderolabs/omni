// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { DecorationItem, HighlighterCore } from 'shiki/core'
import { createHighlighterCore } from 'shiki/core'
import { createJavaScriptRegexEngine } from 'shiki/engine/javascript'

import { createOmniShikiTheme, OMNI_CODE_THEME } from '@/lib/code-theme'

/** Grammars loaded on demand; add one here before using it in a code block. */
const GRAMMARS = {
  shellscript: () => import('@shikijs/langs/shellscript'),
  yaml: () => import('@shikijs/langs/yaml'),
  json: () => import('@shikijs/langs/json'),
} as const

export type CodeLanguage = keyof typeof GRAMMARS

let highlighter: Promise<HighlighterCore> | undefined

/** Keyed by language so concurrent code blocks share a single grammar load. */
const loaded = new Map<CodeLanguage, Promise<void>>()

function getHighlighter() {
  highlighter ??= createHighlighterCore({
    themes: [createOmniShikiTheme()],
    langs: [],
    engine: createJavaScriptRegexEngine(),
  })

  return highlighter
}

/** Class Shiki puts on the parts of the code that match the search query. */
export const SEARCH_MATCH_CLASS = 'code-search-match'

/**
 * Every occurrence of `search` in `code`, as Shiki decorations. Shiki splits
 * tokens at the range boundaries, so a match straddling two tokens still gets
 * marked in full.
 */
function searchDecorations(code: string, search: string) {
  const needle = search.toLowerCase()

  if (!needle) return []

  const haystack = code.toLowerCase()
  const decorations: DecorationItem[] = []

  let start = haystack.indexOf(needle)

  while (start !== -1) {
    const end = start + needle.length

    // Shiki cannot decorate across a line break, and a search box has no
    // business producing a query that spans one.
    if (!code.slice(start, end).includes('\n')) {
      decorations.push({ start, end, properties: { class: SEARCH_MATCH_CLASS } })
    }

    // Resume past the match, so overlapping ones — which Shiki rejects — can't
    // be found.
    start = haystack.indexOf(needle, end)
  }

  return decorations
}

/**
 * Highlights `code` against the Omni theme, returning token spans with no
 * `<pre>`/`<code>` wrapper so the caller keeps control of the layout. Lines are
 * separated by `<br>`, so the markup belongs inside a `white-space: pre` box.
 *
 * Occurrences of `search`, if given, are additionally wrapped in a span with
 * {@link SEARCH_MATCH_CLASS} so the caller can tint them.
 *
 * The output is escaped by Shiki and safe to pass to `v-html`.
 */
export async function highlight(code: string, lang: CodeLanguage, search = '') {
  const shiki = await getHighlighter()

  let grammar = loaded.get(lang)

  if (!grammar) {
    const module = await GRAMMARS[lang]()
    grammar = shiki.loadLanguage(module)
    loaded.set(lang, grammar)
  }

  await grammar

  return shiki.codeToHtml(code, {
    lang,
    theme: OMNI_CODE_THEME,
    structure: 'inline',
    decorations: searchDecorations(code, search),
  })
}
