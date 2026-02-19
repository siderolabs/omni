<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { QuestionMarkCircleIcon } from '@heroicons/vue/24/outline'
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { TalosMountStatusType, TalosRuntimeNamespace } from '@/api/resources'
import type { WatchOptions } from '@/api/watch'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TList from '@/components/common/List/TList.vue'
import TListItem from '@/components/common/List/TListItem.vue'
import { getContext } from '@/context'

enum Encryption {
  Unknown = 'unknown',
  Enabled = 'enabled',
  Disabled = 'disabled',
}

const watchOpts = computed((): WatchOptions => {
  return {
    resource: {
      namespace: TalosRuntimeNamespace,
      type: TalosMountStatusType,
    },
    runtime: Runtime.Talos,
    context: getContext(),
  }
})

const isEncrypted = (item: Resource<{ encrypted?: boolean }>) => {
  if (item.spec.encrypted === undefined) {
    return Encryption.Unknown
  }

  return item.spec.encrypted ? Encryption.Enabled : Encryption.Disabled
}

const encryptionColor = (item: Resource) => {
  switch (isEncrypted(item)) {
    case Encryption.Disabled:
      return 'var(--color-red-r1)'
    case Encryption.Enabled:
      return 'var(--color-green-g1)'
    case Encryption.Unknown:
      return 'var(--color-yellow-y1)'
  }
}
</script>

<template>
  <TList :opts="watchOpts" class="py-4">
    <template #default="{ items }">
      <div
        class="mb-1 grid grid-cols-5 items-center justify-center bg-naturals-n2 px-6 py-2 text-xs"
      >
        <div>Partition</div>
        <div>Filesystem</div>
        <div>Src</div>
        <div>Target</div>
        <div>Encryption</div>
      </div>
      <TListItem v-for="item in items" :key="item.metadata.id!">
        <div class="grid grid-cols-5 items-center justify-center px-3 text-naturals-n12">
          <div class="text-naturals-n14">{{ item.metadata.id }}</div>
          <div>{{ item.spec.filesystemType }}</div>
          <div>{{ item.spec.source }}</div>
          <div>{{ item.spec.target }}</div>
          <div class="flex">
            <div
              class="flex items-center gap-1 rounded bg-naturals-n4 px-2 py-1"
              :style="{ color: encryptionColor(item) }"
            >
              <TIcon
                v-if="isEncrypted(item) === Encryption.Enabled"
                icon="locked"
                class="h-3 w-3"
              />
              <TIcon
                v-else-if="isEncrypted(item) === Encryption.Disabled"
                icon="unlocked"
                class="h-3 w-3"
              />
              <QuestionMarkCircleIcon
                v-else-if="isEncrypted(item) === Encryption.Unknown"
                class="h-4 w-4"
              />
              {{ isEncrypted(item) }}
            </div>
          </div>
        </div>
      </TListItem>
    </template>
  </TList>
</template>
