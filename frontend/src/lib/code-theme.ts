// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { ThemeRegistration } from 'shiki'

export const OMNI_CODE_THEME = 'omni-dark'

/**
 * Green and red are deliberately unused for syntax: in a diff they belong to
 * added and removed lines, and reusing them for tokens muddies that signal.
 * Scalars stay close to the body colour so the eye lands on keys and on the
 * values that actually changed.
 */
export function createOmniCodeTheme(): ThemeRegistration {
  const styles = getComputedStyle(document.documentElement)

  const key = styles.getPropertyValue('--color-blue-b1')
  const scalar = styles.getPropertyValue('--color-naturals-n13')
  const quoted = styles.getPropertyValue('--color-primary-p2')
  const constant = styles.getPropertyValue('--color-yellow-y1')
  const muted = styles.getPropertyValue('--color-naturals-n10')

  return {
    name: OMNI_CODE_THEME,
    type: 'dark',
    colors: {
      'editor.background': styles.getPropertyValue('--color-naturals-n3'),
      'editor.foreground': scalar,
      'editorLineNumber.foreground': styles.getPropertyValue('--color-naturals-n8'),
      'editorLineNumber.activeForeground': styles.getPropertyValue('--color-naturals-n11'),
    },
    tokenColors: [
      {
        scope: ['comment', 'punctuation.definition.comment'],
        settings: {
          foreground: styles.getPropertyValue('--color-naturals-n9'),
          fontStyle: 'italic',
        },
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
        settings: { foreground: styles.getPropertyValue('--color-red-r1') },
      },
    ],
  }
}
