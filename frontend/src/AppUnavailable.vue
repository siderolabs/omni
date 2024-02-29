<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="root">
    <t-header />
    <div class="flex flex-col flex-1 items-center justify-center">
      <div class="error">
        <t-icon icon="warning" class="h-12 w-12 fill-current text-naturals-N14"/>
        <div class="flex flex-col text-naturals-N13">
          <div>
            It appears we are experiencing some issues.
          </div>
          <div>
            Please try later again or <a class="contact-link text-naturals-N12" href="https://www.siderolabs.com/contact/">contact us</a>.
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import THeader from "@/components/THeader/THeader.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";
</script>

<style scoped>
.root {
  @apply flex flex-col;
  min-height: 100vh;
}
.error {
  @apply flex gap-4 bg-naturals-N6 rounded-lg px-6 py-4 drop-shadow-md;
}
.contact-link {
  @apply underline;
}
</style>
