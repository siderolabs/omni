<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
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

onMounted(async () => {
  // Its sad that you cannot execute async function in computed
  const url = '/talosctl/downloads'

  try {
    const res = await fetch(url)
    talosctlRelease.value = await res.json()
  } catch (e) {
    showError(e.message)
    return
  }

  defaultVersion.value = talosctlRelease.value?.release_data.default_version
  selectedVersion.value = defaultVersion.value

  defaultValue.value =
    defaultVersion.value &&
    talosctlRelease.value?.release_data.available_versions[defaultVersion.value].find((item) =>
      item.url.endsWith('linux-amd64'),
    )?.name

  selectedOption.value = defaultValue.value
})

const talosctlRelease = ref<ResponseData>()
const defaultValue = ref<string>()
const selectedOption = ref<string>()
const setOption = (value: string) => (selectedOption.value = value)
const defaultVersion = ref<string>()
const selectedVersion = ref<string>()
const setVersion = (value: string) => (selectedVersion.value = value)

const download = async () => {
  close()

  if (!selectedVersion.value) return

  const link = talosctlRelease.value?.release_data.available_versions[selectedVersion.value].find(
    (item) => item.name === selectedOption.value,
  )
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

  return (
    talosctlRelease.value?.release_data.available_versions[selectedVersion.value]?.map(
      (item) => item.name,
    ) ?? []
  )
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
      <h3 class="text-base text-naturals-N14">Download Talosctl</h3>
      <CloseButton @click="close" />
    </div>

    <div class="mb-5 flex flex-wrap gap-4">
      <div v-if="selectedOption" class="flex flex-wrap gap-4">
        <TSelectList
          title="version"
          :default-value="defaultVersion"
          :values="Object.keys(talosctlRelease?.release_data.available_versions ?? {})"
          :searcheable="true"
          @checked-value="setVersion"
        />
        <TSelectList
          title="talosctl"
          :default-value="defaultValue"
          :values="versionBinaries"
          :searcheable="true"
          @checked-value="setOption"
        />
      </div>
      <div v-else>
        <TSpinner class="h-6 w-6" />
      </div>
    </div>

    <div>
      <p class="text-xs">
        <code>talosctl</code> can be used to access cluster nodes using Talos machine API.
      </p>
      <p class="text-xs">
        More downloads links can be found
        <a
          target="_blank"
          rel="noopener noreferrer"
          class="download-link text-xs"
          href="https://github.com/siderolabs/talos/releases"
          >here</a
        >.
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
.modal-window {
  @apply h-auto w-1/3 p-8;
}

.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-N14;
}

.download-link {
  @apply underline;
}
</style>
