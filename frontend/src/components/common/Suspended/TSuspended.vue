<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TIcon from '@/components/common/Icon/TIcon.vue'
</script>

<template>
  <div class="suspended">
    <div class="suspended__wrapper">
      <TIcon class="suspended__icon" icon="attention" />
      <div class="suspended__content-box">
        This account is currently
        <b>suspended</b>
        and in read-only mode. You can still access your resources but will not be able to make any
        changes.
        <br />
        Please contact support to resolve this issue.
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.suspended {
  @apply flex w-full items-center justify-between rounded border border-l-4 border-red-r2 bg-naturals-n0 px-6 py-4 text-red-r1;
  min-height: 65px;
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
