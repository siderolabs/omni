<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TIcon from '@/components/Icon/TIcon.vue'
</script>

<template>
  <div
    class="flex min-h-16.25 w-full items-center justify-between rounded border border-l-4 border-status-danger-subtle-border bg-surface-page px-base py-compact text-status-danger-text"
  >
    <div class="flex items-center gap-base">
      <TIcon class="size-5 shrink-0" icon="exclamation" />
      <p>
        This account is currently
        <b>suspended</b>
        and in read-only mode. You can still access your resources but will not be able to make any
        changes.
        <br />
        Please contact support to resolve this issue.
      </p>
    </div>
  </div>
</template>
