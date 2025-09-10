<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
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

const selectedOption = ref(options[2])

const download = async () => {
  close()

  const url: string = '/omnictl/' + selectedOption.value

  const a = document.createElement('a')
  a.href = url
  document.body.appendChild(a)
  a.click()
  a.remove()
}

const setOption = (value: string) => {
  selectedOption.value = value
}

const defaultValue = options[2]
</script>

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">Download Omnictl</h3>
      <CloseButton @click="close" />
    </div>

    <div class="mb-5 flex flex-wrap gap-4">
      <TSelectList
        title="omnictl"
        :default-value="defaultValue"
        :values="options"
        :searcheable="true"
        @checked-value="setOption"
      />
    </div>

    <div>
      <p class="text-xs">
        <code>omnictl</code>
        can be used to access omni resources.
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
