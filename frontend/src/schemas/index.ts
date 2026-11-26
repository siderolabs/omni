// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { JSONSchema } from 'monaco-yaml'

// We use $defs, not definitions
type Schema = Omit<JSONSchema, 'definitions'> & { $defs?: Record<string, JSONSchema> }

// Lazy, so each Talos version's schema (~300 kB) is only downloaded once an editor uses it.
const schemas = import.meta.glob<false, string, Schema>('./*.schema.json', {
  import: 'default',
})

export default schemas
