<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">Download Omnictl</h3>
      <close-button @click="close" />
    </div>

    <div class="flex flex-wrap gap-4 mb-5">
      <t-select-list
        @checkedValue="setOption"
        title="omnictl"
        :defaultValue="defaultValue"
        :values="options"
        :searcheable="true"
      />
    </div>

    <div>
      <p class="text-xs"><code>omnictl</code> can be used to access omni resources.</p>
    </div>

    <div class="flex justify-end gap-4 mt-8">
      <t-button @click="close" class="w-32 h-9">
        <span> Cancel </span>
      </t-button>
      <t-button @click="download" class="w-32 h-9">
        <span> Download </span>
      </t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'
import { ref } from 'vue'

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

<style scoped>
.modal-window {
  @apply w-1/3 h-auto p-8;
}

.heading {
  @apply flex justify-between items-center mb-5 text-xl text-naturals-N14;
}
</style>
