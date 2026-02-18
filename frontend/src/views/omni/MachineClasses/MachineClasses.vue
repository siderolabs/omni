<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
import { DefaultNamespace, MachineClassType } from '@/api/resources'
import type { WatchOptions } from '@/api/watch'
import IconButton from '@/components/common/Button/IconButton.vue'
import TButton from '@/components/common/Button/TButton.vue'
import TList from '@/components/common/List/TList.vue'
import TListItem from '@/components/common/List/TListItem.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import Tag from '@/components/common/Tag/Tag.vue'
import TAlert from '@/components/TAlert.vue'
import { canRemoveMachines } from '@/methods/auth'

const watchOpts = computed((): WatchOptions => {
  return {
    resource: {
      namespace: DefaultNamespace,
      type: MachineClassType,
    },
    runtime: Runtime.Omni,
  }
})

const router = useRouter()

const openMachineClassDestroy = (id: string) => {
  router.push({ query: { modal: 'machineClassDestroy', classname: id } })
}
</script>

<template>
  <div class="flex h-full flex-col">
    <div class="flex items-start gap-1">
      <PageHeader title="Machine Classes" class="flex-1" />
      <TButton
        is="router-link"
        :disabled="!canRemoveMachines"
        variant="highlighted"
        :to="{ name: 'MachineClassCreate' }"
      >
        Create Machine Class
      </TButton>
    </div>

    <TList :opts="watchOpts" class="mb-6" search pagination>
      <template #norecords>
        <TAlert type="info" title="No classes found">
          Create your first Machine Class to automate cluster provisioning and scaling from classes
          of machines. Click "Create Machine Class" above or
          <RouterLink class="link-primary" :to="{ name: 'MachineClassCreate' }">here</RouterLink>
          to get started.
        </TAlert>
      </template>

      <template #default="{ items, searchQuery }">
        <div class="header">
          <div class="list-grid">
            <div>Name</div>
            <div>Mode</div>
            <div>Provider</div>
          </div>
        </div>
        <TListItem v-for="item in items" :key="item.metadata.id!">
          <div class="relative pr-3 text-naturals-n12" :class="{ 'pl-7': !item.spec.description }">
            <IconButton
              icon="delete"
              aria-label="delete"
              class="absolute top-0 right-0 bottom-0 my-auto"
              @click="() => openMachineClassDestroy(item.metadata.id!)"
            />
            <div class="list-grid">
              <div>
                <RouterLink
                  :to="{ name: 'MachineClassEdit', params: { classname: item.metadata.id } }"
                  class="list-item-link"
                >
                  <WordHighlighter highlight-class="bg-naturals-n14" :query="searchQuery">
                    {{ item.metadata.id }}
                  </WordHighlighter>
                </RouterLink>
              </div>
              <div class="flex">
                <Tag>
                  {{ item.spec.auto_provision ? 'Auto Provision' : 'Manual' }}
                </Tag>
              </div>
              <div v-if="item.spec.auto_provision">
                {{ item.spec.auto_provision.provider_id }}
              </div>
            </div>
          </div>
        </TListItem>
      </template>
    </TList>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.header {
  @apply mb-1 bg-naturals-n2 px-6 py-2 pl-10 text-xs;
}

.list-grid {
  @apply grid grid-cols-3 items-center justify-center gap-1 pr-12;
}
</style>
