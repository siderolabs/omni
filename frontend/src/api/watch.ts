// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
export const itemID = (item: {
  metadata: { id?: string; name?: string; namespace?: string }
}): string => {
  if (item.metadata === null) {
    return ''
  }

  return `${item.metadata.namespace || 'default'}.${item.metadata.name ?? item.metadata.id}`
}
