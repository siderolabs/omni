<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { QuestionMarkCircleIcon } from '@heroicons/vue/24/outline'
import { LockClosedIcon, LockOpenIcon } from '@heroicons/vue/24/solid'
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { TalosMountStatusType, TalosRuntimeNamespace } from '@/api/resources'
import type { WatchOptions } from '@/api/watch'
import TList from '@/components/common/List/TList.vue'
import TListItem from '@/components/common/List/TListItem.vue'
import { getContext } from '@/context'
import { green, red, yellow } from '@/vars/colors'

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
      return red.R1
    case Encryption.Enabled:
      return green.G1
    case Encryption.Unknown:
      return yellow.Y1
  }
}
</script>

<template>
  <TList :opts="watchOpts">
    <template #default="{ items }">
      <div class="header list-grid">
        <div>Partition</div>
        <div>Filesystem</div>
        <div>Src</div>
        <div>Target</div>
        <div>Encryption</div>
      </div>
      <TListItem v-for="item in items" :key="item.metadata.id!">
        <div class="list-grid px-3 text-naturals-N12">
          <div class="text-naturals-N14">{{ item.metadata.id }}</div>
          <div>{{ item.spec.filesystemType }}</div>
          <div>{{ item.spec.source }}</div>
          <div>{{ item.spec.target }}</div>
          <div class="flex">
            <div
              class="flex items-center gap-1 rounded bg-naturals-N4 px-2 py-1"
              :style="{ color: encryptionColor(item) }"
            >
              <LockClosedIcon v-if="isEncrypted(item) === Encryption.Enabled" class="h-3 w-3" />
              <LockOpenIcon v-else-if="isEncrypted(item) === Encryption.Disabled" class="h-3 w-3" />
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

<style scoped>
.list-grid {
  @apply grid grid-cols-5 items-center justify-center;
}

.header {
  @apply mb-1 bg-naturals-N2 px-6 py-2 text-xs;
}
</style>
