<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <t-list :opts="watchOpts">
    <template #default="{ items }">
      <div class="header list-grid">
        <div>Partition</div>
        <div>Filesystem</div>
        <div>Src</div>
        <div>Target</div>
        <div>Encryption</div>
      </div>
      <t-list-item v-for="item in items" :key="item.metadata.id!">
        <div class="px-3 list-grid text-naturals-N12">
          <div class="text-naturals-N14">{{ item.metadata.id }}</div>
          <div>{{ item.spec.filesystemType }}</div>
          <div>{{ item.spec.source }}</div>
          <div>{{ item.spec.target }}</div>
          <div class="flex">
            <div
              class="flex gap-1 items-center rounded bg-naturals-N4 px-2 py-1"
              :style="{ color: encryptionColor(item) }"
            >
              <LockClosedIcon v-if="isEncrypted(item) === Encryption.Enabled" class="w-3 h-3" />
              <LockOpenIcon v-else-if="isEncrypted(item) === Encryption.Disabled" class="w-3 h-3" />
              <QuestionMarkCircleIcon
                v-else-if="isEncrypted(item) === Encryption.Unknown"
                class="w-4 h-4"
              />
              {{ isEncrypted(item) }}
            </div>
          </div>
        </div>
      </t-list-item>
    </template>
  </t-list>
</template>

<script setup lang="ts">
import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { TalosMountStatusType, TalosRuntimeNamespace } from '@/api/resources'
import type { WatchOptions } from '@/api/watch'
import TList from '@/components/common/List/TList.vue'
import TListItem from '@/components/common/List/TListItem.vue'
import { getContext } from '@/context'
import { QuestionMarkCircleIcon } from '@heroicons/vue/24/outline'
import { LockClosedIcon, LockOpenIcon } from '@heroicons/vue/24/solid'
import { computed } from 'vue'
import { red, green, yellow } from '@/vars/colors'

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

<style scoped>
.list-grid {
  @apply grid grid-cols-5 items-center justify-center;
}

.header {
  @apply bg-naturals-N2 mb-1 py-2 px-6 text-xs;
}
</style>
