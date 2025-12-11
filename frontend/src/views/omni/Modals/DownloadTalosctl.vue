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
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import { getDocsLink, getPlatform } from '@/methods'
import { useTalosctlDownloads } from '@/methods/useTalosctlDownloads'
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

const platform = computedAsync(getPlatform)

const { downloads: availableVersions, defaultVersion } = useTalosctlDownloads()

const defaultPlatform = computed(() => {
  if (!defaultVersion.value || !platform.value) return

  const [os, arch] = platform.value

  const assets = availableVersions.value?.get(defaultVersion.value)
  const defaultAsset = assets?.find((item) => item.url.endsWith('linux-amd64'))
  const preferredAsset = assets?.find((item) => item.url.endsWith(`${os}-${arch}`))

  return (preferredAsset ?? defaultAsset)?.name
})

const selectedVersion = ref<string>()
const selectedPlatform = ref<string>()

const download = () => {
  close()

  if (!selectedVersion.value) return

  const link = availableVersions.value
    ?.get(selectedVersion.value)
    ?.find((item) => item.name === selectedPlatform.value)
  if (!link) {
    return
  }

  const a = document.createElement('a')
  a.href = link.url
  document.body.appendChild(a)
  a.click()
  a.remove()
}

const versionBinaries = computed<string[]>(() => {
  if (!selectedVersion.value) return []

  return availableVersions.value?.get(selectedVersion.value)?.map((item) => item.name) ?? []
})
</script>

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">Download Talosctl</h3>
      <CloseButton @click="close" />
    </div>

    <p class="mb-5 text-xs">
      <code>talosctl</code>
      can be used to access cluster nodes using Talos machine API. Read the
      <a
        class="link-primary"
        target="_blank"
        rel="noopener noreferrer"
        :href="getDocsLink('omni', '/getting-started/how-to-install-talosctl')"
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

    <div class="mb-5 flex flex-wrap gap-4">
      <div v-if="availableVersions && platform" class="flex flex-wrap gap-4">
        <TSelectList
          v-model="selectedVersion"
          title="version"
          :default-value="defaultVersion"
          :values="Array.from(availableVersions.keys())"
          searcheable
        />
        <TSelectList
          v-model="selectedPlatform"
          title="talosctl"
          :default-value="defaultPlatform"
          :values="versionBinaries"
          searcheable
        />
      </div>
      <div v-else>
        <TSpinner class="h-6 w-6" />
      </div>
    </div>

    <p class="flex text-xs">
      More downloads links can be found&nbsp;
      <a
        target="_blank"
        rel="noopener noreferrer"
        class="link-primary"
        href="https://github.com/siderolabs/talos/releases"
      >
        here
      </a>
      .
    </p>

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
