// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import type { InstallImageInfo } from '@/api/omni/specs/omni.pb'

const fields = [
  ['talosVersion', 'talos_version'],
  ['schematic', 'schematic_id'],
  ['imageFactoryHost', 'image_factory_host'],
] as const

function formatValue(key: string, value?: string) {
  return `${key}: ${value || '""'}`
}

// installImageDiff builds a headerless unified diff hunk of an install image change, so that it is
// shown by the same DiffRenderer as the machine config diffs. Fields empty on both sides are left
// out, e.g., the image factory host of the upgrades recorded by older Omni versions.
export function installImageDiff(from: InstallImageInfo = {}, to: InstallImageInfo = {}) {
  const shown = fields.filter(([, field]) => from[field] || to[field])

  const lines = shown.flatMap(([key, field]) => {
    const before = formatValue(key, from[field])
    const after = formatValue(key, to[field])

    return before === after ? [` ${before}`] : [`-${before}`, `+${after}`]
  })

  return [`@@ -1,${shown.length} +1,${shown.length} @@`, ...lines].join('\n')
}
