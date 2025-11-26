<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { TalosExtensionsSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, TalosExtensionsType } from '@/api/resources'
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import type { FormState } from '@/views/omni/InstallationMedia/InstallationMediaCreate.vue'

const formState = defineModel<FormState>({ required: true })
const filterExtensions = ref('')

const { data } = useResourceWatch<TalosExtensionsSpec>(() => ({
  resource: {
    id: formState.value.talosVersion!,
    type: TalosExtensionsType,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni,
}))

const filteredExtensions = computed(() => {
  const items = data.value?.spec.items

  if (!items || !filterExtensions.value) return items

  return items.filter((item) => item.name?.includes(filterExtensions.value))
})

function toggleExtension(ref: string, enabled: boolean) {
  const set = new Set(formState.value.systemExtensions)

  if (enabled) {
    set.add(ref)
  } else {
    set.delete(ref)
  }

  formState.value.systemExtensions = Array.from(set)
}
</script>

<template>
  <div class="flex flex-col gap-4 overflow-hidden">
    <span class="text-sm font-medium text-naturals-n14">System Extensions</span>

    <p class="text-xs">
      <a
        href="https://github.com/siderolabs/extensions"
        target="_blank"
        rel="noopener noreferrer"
        class="link-primary"
      >
        System Extensions
      </a>
      enhance the Talos Linux base image with additional features such as extra drivers, hardware
      firmware, container runtimes, guest agents, and more.
      <br />
      Select the extensions needed for your environment. If unsure, you can skip this step.
    </p>

    <TInput v-model="filterExtensions" placeholder="Search" icon="search" />

    <div class="flex flex-col gap-4 overflow-auto">
      <TCheckbox
        v-for="item in filteredExtensions"
        :key="item.name"
        :model-value="formState.systemExtensions?.includes(item.name!)"
        @update:model-value="(value) => toggleExtension(item.name!, value)"
      >
        <div class="flex flex-col">
          <span class="font-medium text-naturals-n14">
            {{ item.name }} {{ `(${item.version})` }}
          </span>
          <span>{{ item.description }}</span>
        </div>
      </TCheckbox>
    </div>
  </div>
</template>
