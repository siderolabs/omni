<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="suspended">
    <div class="suspended__wrapper">
      <t-icon class="suspended__icon" icon="attention" />
      <div class="suspended__content-box">
        This account is currently <b>suspended</b> and in read-only mode. You can still access your
        resources but will not be able to make any changes.<br />
        Please contact support to resolve this issue.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import TIcon from '@/components/common/Icon/TIcon.vue'
</script>

<style scoped>
.suspended {
  @apply text-red-R1 border-l-4 w-full flex justify-between items-center py-4 px-6 bg-naturals-N0 rounded border border-naturals-N5;
  min-height: 65px;
  border-color: rgba(110, 47, 48, var(--tw-border-opacity));
}
.suspended__wrapper {
  @apply flex items-center;
}
.suspended__icon {
  @apply mr-5 fill-current;
  width: 20px;
  height: 20px;
}
</style>
