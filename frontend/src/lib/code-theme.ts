// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type * as monaco from 'monaco-editor'
import type { ThemeRegistration } from 'shiki'

import type { Theme } from '@/methods/theme'

export const OMNI_CODE_THEME = 'omni'

/**
 * Colours come from the design system's syntax roles, which keep green and red
 * out of syntax so a token never reads as a diff change.
 */
const ROLES = {
  key: '--color-syntax-key',
  scalar: '--color-syntax-plain',
  quoted: '--color-syntax-string',
  constant: '--color-syntax-constant',
  keyword: '--color-syntax-keyword',
  muted: '--color-syntax-punctuation',
  comment: '--color-syntax-comment',
  invalid: '--color-syntax-invalid',
  match: '--color-highlight-match',
  matchBorder: '--color-highlight-match-border',

  panel: '--color-surface-card',
  popover: '--color-surface-raised',
  canvas: '--color-surface-page',
  field: '--color-surface-chrome',
  border: '--color-border-default',
  lineNumber: '--color-content-muted',
  activeLineNumber: '--color-content-secondary',
} as const

type Palette = Record<keyof typeof ROLES, string>

function palette(read: (property: string) => string) {
  return Object.fromEntries(
    Object.entries(ROLES).map(([role, property]) => [role, read(property)]),
  ) as Palette
}

/** Appends an alpha channel to a `#rrggbb` colour, for Monaco's hex-only colours. */
function withAlpha(hex: string, alpha: number) {
  const channel = Math.round(alpha * 255)
    .toString(16)
    .padStart(2, '0')

  return `${hex}${channel}`
}

/**
 * Shiki writes colours into inline styles, so referencing the variables lets
 * highlighted code follow the page between light and dark without re-rendering.
 */
export function createOmniShikiTheme(): ThemeRegistration {
  const { key, scalar, quoted, constant, keyword, muted, comment, invalid, ...chrome } = palette(
    (property) => `var(${property})`,
  )

  return {
    name: OMNI_CODE_THEME,
    // Leaving static for simplicity, colours are overriden through variables anyway
    type: 'dark',
    colors: {
      'editor.background': chrome.panel,
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
        settings: { foreground: keyword },
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
 *
 * Monaco only accepts bare hex, so the values are read from the page as it is
 * now: define the theme again after the page theme changes.
 */
export function createOmniMonacoTheme(theme: Theme): monaco.editor.IStandaloneThemeData {
  const styles = getComputedStyle(document.documentElement)
  const { key, scalar, constant, keyword, muted, comment, invalid, ...chrome } = palette(
    (property) => styles.getPropertyValue(property).trim(),
  )

  return {
    base: theme === 'light' ? 'vs' : 'vs-dark',
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
      { token: 'tag', foreground: keyword },
      { token: 'namespace', foreground: keyword },
      { token: 'operators', foreground: muted },
      { token: 'delimiter', foreground: muted },
      { token: 'meta.directive', foreground: muted },
    ],
    colors: {
      'dropdown.background': chrome.popover,

      'editorStickyScroll.background': chrome.canvas,

      'editor.background': chrome.canvas,
      'editor.foreground': scalar,

      'editorLineNumber.foreground': chrome.lineNumber,
      'editorLineNumber.activeForeground': chrome.activeLineNumber,

      'editorHoverWidget.background': chrome.popover,
      'editorHoverWidget.border': chrome.border,

      'editorOverviewRuler.border': '#00000000',

      'editorWidget.background': chrome.popover,
      'editorWidget.border': chrome.border,

      'editor.findMatchBackground': withAlpha(chrome.matchBorder, 0.35),
      'editor.findMatchBorder': chrome.matchBorder,
      'editor.findMatchHighlightBackground': withAlpha(chrome.match, 0.8),
      'editor.findMatchHighlightBorder': chrome.matchBorder,

      'input.background': chrome.field,
      'input.border': chrome.border,
    },
  }
}
