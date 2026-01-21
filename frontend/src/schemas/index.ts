// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { JSONSchema } from 'monaco-yaml'

// We use $defs, not definitions
type Schema = Omit<JSONSchema, 'definitions'> & { $defs?: Record<string, JSONSchema> }

const schemas = import.meta.glob<true, string, Schema>('./*.schema.json', {
  import: 'default',
  eager: true,
})

export default schemas
