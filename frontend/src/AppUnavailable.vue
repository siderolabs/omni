<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TIcon from '@/components/common/Icon/TIcon.vue'
import THeader from '@/components/THeader/THeader.vue'
</script>

<template>
  <div class="flex min-h-screen flex-col">
    <THeader />
    <div class="flex flex-1 flex-col items-center justify-center">
      <div class="flex gap-4 rounded-lg bg-naturals-n6 px-6 py-4 drop-shadow-md">
        <TIcon icon="warning" class="h-12 w-12 fill-current text-naturals-n14" />
        <div class="flex flex-col text-naturals-n13">
          <div>It appears we are experiencing some issues.</div>
          <div class="flex">
            Please try later again or&nbsp;
            <a class="text-naturals-n12 underline" href="https://www.siderolabs.com/contact/">
              contact us
            </a>
            .
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
