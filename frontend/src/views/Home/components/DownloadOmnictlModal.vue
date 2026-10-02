<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computedAsync } from '@vueuse/core'
import { computed, ref } from 'vue'

import CodeBlock from '@/components/CodeBlock/CodeBlock.vue'
import Modal from '@/components/Modals/Modal.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import { getDocsLink, getPlatform } from '@/methods'

const open = defineModel<boolean>('open', { default: false })

const options = [
  'omnictl-darwin-amd64',
  'omnictl-darwin-arm64',
  'omnictl-linux-amd64',
  'omnictl-linux-arm64',
  'omnictl-windows-amd64.exe',
]

const platform = computedAsync(getPlatform)

const defaultValue = computed(() => {
  if (!platform.value) return options[2]

  const [os, arch] = platform.value

  return options.find((o) => o === `omnictl-${os}-${arch}`) ?? options[2]
})

const selectedOption = ref<string>()
</script>

<template>
  <Modal
    v-model:open="open"
    title="Download Omnictl"
    action-label="Download"
    :action-disabled="!selectedOption"
    :action-href="`/api/omnictl/${encodeURI(selectedOption!)}`"
    @confirm="open = false"
  >
    <p class="mb-base text-xs">
      <code>omnictl</code>
      can be used to access omni resources. Read the
      <a
        class="link-primary"
        target="_blank"
        rel="noopener noreferrer"
        :href="getDocsLink('omni', '/getting-started/install-and-configure-omnictl')"
      >
        docs
      </a>
      for more information.
    </p>

    <div class="mb-base flex flex-col gap-tight">
      <span class="text-xs text-content-emphasis">macOS and Linux (recommended)</span>
      <CodeBlock code="brew install siderolabs/tap/sidero-tools" />
    </div>

    <span class="mb-tight text-xs text-content-emphasis">Manual installation</span>

    <div v-if="platform" class="flex flex-wrap gap-compact">
      <TSelectList
        v-model="selectedOption"
        title="omnictl"
        :default-value="defaultValue"
        :values="options"
        searcheable
      />
    </div>
  </Modal>
</template>
