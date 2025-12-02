<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computedAsync } from '@vueuse/core'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import CodeBlock from '@/components/common/CodeBlock/CodeBlock.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import { getDocsLink, getPlatform } from '@/methods'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const router = useRouter()
let closed = false

const close = () => {
  if (closed) {
    return
  }

  closed = true

  router.go(-1)
}

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

const selectedOption = ref(defaultValue)

const download = () => {
  close()

  const a = document.createElement('a')
  a.href = `/omnictl/${encodeURI(selectedOption.value)}`
  document.body.appendChild(a)
  a.click()
  a.remove()
}
</script>

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">Download Omnictl</h3>
      <CloseButton @click="close" />
    </div>

    <p class="mb-5 text-xs">
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

    <div class="mb-5 flex flex-col gap-2">
      <span class="text-xs text-naturals-n14">macOS and Linux (recommended)</span>
      <CodeBlock code="brew install siderolabs/tap/sidero-tools" />
    </div>

    <span class="mb-2 text-xs text-naturals-n14">Manual installation</span>

    <div v-if="platform" class="flex flex-wrap gap-4">
      <TSelectList
        v-model="selectedOption"
        title="omnictl"
        :default-value="defaultValue"
        :values="options"
        searcheable
      />
    </div>

    <div class="mt-8 flex justify-end gap-4">
      <TButton class="h-9 w-32" @click="close">
        <span>Cancel</span>
      </TButton>
      <TButton class="h-9 w-32" @click="download">
        <span>Download</span>
      </TButton>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.modal-window {
  @apply h-auto w-1/3 p-8;
}

.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-n14;
}
</style>
