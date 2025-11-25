<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computedAsync } from '@vueuse/core'
import { compare } from 'semver'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import { getPlatform } from '@/methods'
import { showError } from '@/notification'
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

const talosctlRelease = computedAsync(async () => {
  try {
    const res = await fetch('/talosctl/downloads')
    return (await res.json()) as ResponseData
  } catch (e) {
    showError(e.message)
  }
})

const platform = computedAsync(getPlatform)

const availableVersions = computed(
  () =>
    new Map(
      Object.entries(talosctlRelease.value?.release_data.available_versions ?? {}).sort(
        ([a], [b]) => compare(a.replace('v', ''), b.replace('v', '')),
      ),
    ),
)

const defaultVersion = computed(() => Array.from(availableVersions.value.keys()).pop())
const defaultPlatform = computed(() => {
  if (!defaultVersion.value || !platform.value) return

  const [os, arch] = platform.value

  const assets = availableVersions.value.get(defaultVersion.value)
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
    .get(selectedVersion.value)
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

  return availableVersions.value.get(selectedVersion.value)?.map((item) => item.name) ?? []
})

interface ResponseData {
  status: string
  release_data: ReleaseData
}

interface ReleaseData {
  default_version: string
  available_versions: { [key: string]: Asset[] }
}

interface Asset {
  name: string
  url: string
}
</script>

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">Download Talosctl</h3>
      <CloseButton @click="close" />
    </div>

    <div class="mb-5 flex flex-wrap gap-4">
      <div v-if="talosctlRelease && platform" class="flex flex-wrap gap-4">
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

    <div>
      <p class="text-xs">
        <code>talosctl</code>
        can be used to access cluster nodes using Talos machine API.
      </p>
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
