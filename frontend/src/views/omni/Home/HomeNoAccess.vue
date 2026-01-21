<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TButton from '@/components/common/Button/TButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import { downloadOmniconfig } from '@/methods'
</script>

<template>
  <div class="flex h-full flex-col items-center justify-center gap-2">
    <div class="relative mb-6 h-16 w-16">
      <div
        class="absolute top-0 left-0 h-full w-full translate-x-1.5 -translate-y-1.5 rotate-12 rounded-lg bg-naturals-n2"
      />

      <div
        class="absolute top-0 left-0 flex h-full w-full items-center justify-center rounded-lg bg-naturals-n3"
      >
        <TIcon icon="warning" class="h-6 w-6 text-naturals-n11" />
      </div>
    </div>

    <p class="text-lg text-naturals-n14">You don't have access to Omni Web</p>
    <p class="text-xs text-naturals-n10">At least Reader role is required</p>

    <div class="mt-3 flex gap-3">
      <TButton type="primary" icon="talos-config" icon-position="left" @click="downloadOmniconfig">
        Download
        <code>omniconfig</code>
      </TButton>

      <TButton
        type="primary"
        icon="talos-config"
        icon-position="left"
        @click="$router.push({ query: { modal: 'downloadOmnictlBinaries' } })"
      >
        Download omnictl
      </TButton>
    </div>
  </div>
</template>
