<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TButton from '@/components/common/Button/TButton.vue'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const { open, title = 'Confirm Action' } = defineProps<{
  open?: boolean
  title?: string
}>()

defineEmits<{
  close: []
  confirm: []
}>()
</script>

<template>
  <div
    class="fixed inset-0 z-30 flex items-center justify-center bg-naturals-n0/90"
    :class="!open && 'hidden'"
  >
    <div class="modal-window w-auto gap-4">
      <div class="flex items-start justify-between">
        <h3 class="text-base text-naturals-n14">{{ title }}</h3>

        <CloseButton @click="$emit('close')" />
      </div>

      <p><slot></slot></p>

      <div class="flex items-center justify-end gap-2">
        <TButton @click="$emit('close')">Close</TButton>
        <TButton type="highlighted" @click="$emit('confirm')">Confirm</TButton>
      </div>
    </div>
  </div>
</template>
