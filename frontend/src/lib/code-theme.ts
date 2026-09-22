// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type * as monaco from 'monaco-editor'
import type { ThemeRegistration } from 'shiki'

export const OMNI_CODE_THEME = 'omni-dark'

/**
 * Green and red are deliberately unused for syntax: in a diff they belong to
 * added and removed lines, and reusing them for tokens muddies that signal.
 * Scalars stay close to the body colour so the eye lands on keys and on the
 * values that actually changed.
 */
function palette() {
  const styles = getComputedStyle(document.documentElement)

  // Monaco only accepts bare hex, so whitespace around the value has to go.
  const read = (name: string) => styles.getPropertyValue(name).trim()

  return {
    key: read('--color-blue-b1'),
    scalar: read('--color-naturals-n13'),
    quoted: read('--color-primary-p2'),
    constant: read('--color-yellow-y1'),
    muted: read('--color-naturals-n10'),
    comment: read('--color-naturals-n9'),
    invalid: read('--color-red-r1'),

    surface: read('--color-naturals-n3'),
    canvas: read('--color-naturals-n0'),
    field: read('--color-naturals-n1'),
    border: read('--color-naturals-n7'),
    lineNumber: read('--color-naturals-n8'),
    activeLineNumber: read('--color-naturals-n11'),
  }
}

export function createOmniShikiTheme(): ThemeRegistration {
  const { key, scalar, quoted, constant, muted, comment, invalid, ...chrome } = palette()

  return {
    name: OMNI_CODE_THEME,
    type: 'dark',
    colors: {
      'editor.background': chrome.surface,
      'editor.foreground': scalar,
      'editorLineNumber.foreground': chrome.lineNumber,
      'editorLineNumber.activeForeground': chrome.activeLineNumber,
    },
    tokenColors: [
      {
        scope: ['comment', 'punctuation.definition.comment'],
        settings: { foreground: comment, fontStyle: 'italic' },
      },
      {
        // YAML mapping keys, plus the equivalent scope in JSON.
        scope: ['entity.name.tag', 'support.type.property-name', 'meta.object-literal.key'],
        settings: { foreground: key },
      },
      {
        scope: ['string.quoted', 'string.template', 'constant.other.symbol'],
        settings: { foreground: quoted },
      },
      {
        scope: ['string.unquoted', 'string'],
        settings: { foreground: scalar },
      },
      {
        scope: [
          'constant.numeric',
          'constant.language',
          'constant.language.boolean',
          'constant.language.null',
        ],
        settings: { foreground: constant },
      },
      {
        // Anchors, aliases, explicit tags and block scalar indicators (| and >).
        scope: [
          'entity.name.type.anchor',
          'variable.other.alias',
          'storage.type.tag-handle',
          'keyword.control.flow.block-scalar',
        ],
        settings: { foreground: constant },
      },
      {
        scope: ['punctuation', 'keyword.operator', 'meta.separator'],
        settings: { foreground: muted },
      },
      {
        scope: 'invalid',
        settings: { foreground: invalid },
      },
    ],
  }
}

/**
 * The same palette as {@link createOmniShikiTheme}, expressed against Monaco's
 * Monarch token names rather than TextMate scopes. Monarch cannot tell a quoted
 * scalar from a plain one — both are `string` — so quoted values get the plain
 * scalar colour here instead of the diff viewer's `quoted`.
 */
export function createOmniMonacoTheme(): monaco.editor.IStandaloneThemeData {
  const { key, scalar, constant, muted, comment, invalid, ...chrome } = palette()

  return {
    base: 'vs-dark',
    inherit: true,
    rules: [
      { token: 'comment', foreground: comment, fontStyle: 'italic' },
      // Mapping keys, in both block and flow collections.
      { token: 'type', foreground: key },
      { token: 'string', foreground: scalar },
      { token: 'string.invalid', foreground: invalid },
      { token: 'string.escape.invalid', foreground: invalid },
      { token: 'number', foreground: constant },
      // `true`, `false`, `null` and friends.
      { token: 'keyword', foreground: constant },
      // Explicit tags (`!!str`) and anchors/aliases (`&a`, `*a`).
      { token: 'tag', foreground: constant },
      { token: 'namespace', foreground: constant },
      { token: 'operators', foreground: muted },
      { token: 'delimiter', foreground: muted },
      { token: 'meta.directive', foreground: muted },
    ],
    colors: {
      'dropdown.background': chrome.surface,

      'editorStickyScroll.background': chrome.canvas,

      'editor.background': chrome.canvas,
      'editor.foreground': scalar,

      'editorLineNumber.foreground': chrome.lineNumber,
      'editorLineNumber.activeForeground': chrome.activeLineNumber,

      'editorHoverWidget.background': chrome.surface,
      'editorHoverWidget.border': chrome.border,

      'editorOverviewRuler.border': '#00000000',

      'editorWidget.background': chrome.surface,
      'editorWidget.border': chrome.border,

      'input.background': chrome.field,
      'input.border': chrome.border,
    },
  }
}
