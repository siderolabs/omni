// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import type { Resource } from '@/api/grpc'

/**
 * COSI's meta.ResourceDefinitionSpec. It has no protobuf definition, so it is
 * serialized through its YAML tags rather than generated.
 */
export interface ResourceDefinitionSpec {
  type?: string
  displayType?: string
  defaultNamespace?: string
  aliases?: string[]
  allAliases?: string[]
  sensitivity?: string
}

/** ResourceDefinition IDs are the lowercased resource type. */
export function resourceDefinitionID(type: string) {
  return type.toLowerCase()
}

/**
 * Removes fields the frontend adds for its own bookkeeping, such as the sort key
 * added by useResourceWatch, so resources are shown and exported as the API sent them.
 */
export function withoutClientFields<T extends Resource>(resource: T): T {
  const copy: T & { sortFieldData?: string } = { ...resource }
  delete copy.sortFieldData

  return copy
}
