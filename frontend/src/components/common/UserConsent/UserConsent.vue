<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TButton from '@/components/common/Button/TButton.vue'

defineEmits<{
  decline: []
  accept: []
}>()
</script>

<template>
  <div class="absolute top-0 left-0 z-50 flex h-screen w-screen flex-col justify-stretch">
    <div class="flex-1 bg-black opacity-25" />
    <div class="border-t border-primary-p3 bg-naturals-n6 p-4">
      <p class="text-lg text-naturals-n13">Cookies for a Better Experience</p>
      <div class="flex items-center justify-between">
        <p class="text-sm">
          We use cookies to improve our website's interface and onboarding experience. We don't use
          them for ads or share your data with third parties.
          <a
            class="link-primary"
            rel="noopener noreferrer"
            href="https://www.siderolabs.com/privacy-policy/"
            target="_blank"
          >
            Learn more
          </a>
        </p>
        <div class="flex gap-2">
          <TButton @click="$emit('decline')">Decline</TButton>
          <TButton type="highlighted" @click="$emit('accept')">Accept</TButton>
        </div>
      </div>
    </div>
  </div>
</template>
