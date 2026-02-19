<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { TalosMountStatusType, TalosRuntimeNamespace } from '@/api/resources'
import { itemID } from '@/api/watch'
import TIcon, { type IconType } from '@/components/common/Icon/TIcon.vue'
import TableCell from '@/components/common/Table/TableCell.vue'
import TableRoot from '@/components/common/Table/TableRoot.vue'
import TableRow from '@/components/common/Table/TableRow.vue'
import { getContext } from '@/context'
import { useResourceWatch } from '@/methods/useResourceWatch'

enum Encryption {
  Unknown = 'unknown',
  Enabled = 'enabled',
  Disabled = 'disabled',
}

interface TalosMountStatusSpec {
  encrypted?: boolean
  filesystemType?: string
  source?: string
  target?: string
}

const { data: items } = useResourceWatch<TalosMountStatusSpec>({
  resource: {
    namespace: TalosRuntimeNamespace,
    type: TalosMountStatusType,
  },
  runtime: Runtime.Talos,
  context: getContext(),
})

const isEncrypted = (item: Resource<TalosMountStatusSpec>) => {
  if (item.spec.encrypted === undefined) {
    return Encryption.Unknown
  }

  return item.spec.encrypted ? Encryption.Enabled : Encryption.Disabled
}

const encryptionClass = (item: Resource<TalosMountStatusSpec>) => {
  switch (isEncrypted(item)) {
    case Encryption.Disabled:
      return 'text-red-r1'
    case Encryption.Enabled:
      return 'text-green-g1'
    case Encryption.Unknown:
      return 'text-yellow-y1'
  }
}

const encryptionIcon = (item: Resource<TalosMountStatusSpec>): IconType => {
  switch (isEncrypted(item)) {
    case Encryption.Disabled:
      return 'unlocked'
    case Encryption.Enabled:
      return 'locked'
    case Encryption.Unknown:
      return 'question-mark-circle'
  }
}
</script>

<template>
  <TableRoot class="my-4 w-full">
    <template #head>
      <TableRow>
        <TableCell th>Partition</TableCell>
        <TableCell th>Filesystem</TableCell>
        <TableCell th>Src</TableCell>
        <TableCell th>Target</TableCell>
        <TableCell th>Encryption</TableCell>
      </TableRow>
    </template>

    <template #body>
      <TableRow v-for="item in items" :key="itemID(item)">
        <TableCell class="text-naturals-n14">{{ item.metadata.id }}</TableCell>
        <TableCell>{{ item.spec.filesystemType }}</TableCell>
        <TableCell>{{ item.spec.source }}</TableCell>
        <TableCell>{{ item.spec.target }}</TableCell>
        <TableCell :class="encryptionClass(item)">
          <span class="inline-flex items-center gap-1 rounded bg-naturals-n4 px-2 py-1">
            <TIcon
              :icon="encryptionIcon(item)"
              :class="isEncrypted(item) === Encryption.Unknown ? 'size-4' : 'size-3'"
            />

            {{ isEncrypted(item) }}
          </span>
        </TableCell>
      </TableRow>
    </template>
  </TableRoot>
</template>
